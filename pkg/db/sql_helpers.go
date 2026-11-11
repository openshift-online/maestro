package db

import (
	"reflect"
	"strings"

	"github.com/jinzhu/inflection"
	"github.com/yaacov/tree-search-language/pkg/tsl"
	"gorm.io/gorm"

	"github.com/openshift-online/maestro/pkg/errors"
)

// getField gets the sql field associated with a name.
func getField(name string, disallowedFields map[string]string) (field string, err *errors.ServiceError) {
	// We want to accept names with trailing and leading spaces
	trimmedName := strings.Trim(name, " ")

	// Check for nested field, e.g., subscription_labels.key
	checkName := trimmedName
	fieldParts := strings.Split(trimmedName, ".")
	if len(fieldParts) > 2 {
		err = errors.BadRequest("a search field is not valid")
		return
	}
	for _, part := range fieldParts {
		if !isSQLIdentifier(part) {
			err = errors.BadRequest("a search field is not valid")
			return
		}
	}
	if len(fieldParts) > 1 {
		checkName = fieldParts[1]
	}

	// Check for allowed fields
	_, ok := disallowedFields[checkName]
	if ok {
		err = errors.BadRequest("a search field is not valid")
		return
	}
	field = trimmedName
	return
}

func isSQLIdentifier(value string) bool {
	if value == "" {
		return false
	}
	for i, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || r == '_' ||
			(i > 0 && r >= '0' && r <= '9') {
			continue
		}
		return false
	}
	return true
}

// FieldNameWalk walks on the filter tree and check/replace
// the search fields names:
// a. the the field name is valid.
// b. replace the field name with the SQL column name.
func FieldNameWalk(
	n tsl.Node,
	disallowedFields map[string]string) (newNode tsl.Node, err *errors.ServiceError) {

	var field string
	var l, r tsl.Node

	switch n.Func {
	case tsl.IdentOp:
		// If this is an Identifier, check field name is a string.
		userFieldName, ok := n.Left.(string)
		if !ok {
			err = errors.BadRequest("Identifier name must be a string")
			return
		}

		// Check field name in the disallowedFields field names.
		field, err = getField(userFieldName, disallowedFields)
		if err != nil {
			return
		}

		// Replace identifier name.
		newNode = tsl.Node{Func: tsl.IdentOp, Left: field}
	case tsl.StringOp, tsl.NumberOp:
		// This are leafs, just return.
		newNode = tsl.Node{Func: n.Func, Left: n.Left}
	default:
		// o/w continue walking the tree.
		if n.Left != nil {
			l, err = FieldNameWalk(n.Left.(tsl.Node), disallowedFields)
			if err != nil {
				return
			}
		}

		// Add right child(ren) if exist.
		if n.Right != nil {
			switch v := n.Right.(type) {
			case tsl.Node:
				// It's a regular node, just add it.
				r, err = FieldNameWalk(v, disallowedFields)
				if err != nil {
					return
				}

				newNode = tsl.Node{Func: n.Func, Left: l, Right: r}
			case []tsl.Node:
				// It's a list of nodes, some TSL operators have multiple RHS arguments
				// for example `IN` and `BETWEEN`. If this operator has a list of arguments,
				// loop over the list, and add all nodes.
				var rr []tsl.Node

				// Add all nodes in the right side array.
				for _, e := range v {
					r, err = FieldNameWalk(e, disallowedFields)
					if err != nil {
						return
					}

					rr = append(rr, r)
				}

				newNode = tsl.Node{Func: n.Func, Left: l, Right: rr}
			default:
				// We only support `Node` and `[]Node` types for the right hand side,
				// of TSL operators. If here than this is an unsupported right hand side
				// type.
				err = errors.BadRequest("unsupported right hand side type in search query")
			}
		} else {
			// If here than `n.Right` is nil. This is a legit type of node,
			// we just need to ignore the right hand side, and continue walking the
			// tree.
			newNode = tsl.Node{Func: n.Func, Left: l}
		}
	}

	return
}

// OrderByField contains a validated field and direction. Keeping these
// separate prevents callers from passing a user-built SQL fragment to GORM.
type OrderByField struct {
	Field      string
	Descending bool
}

// ArgsToOrderBy parses orderBy arguments without constructing SQL. The DAO
// resolves each field against its model schema and passes it to GORM as a
// structured column.
func ArgsToOrderBy(
	orderByArgs []string,
	disallowedFields map[string]string) (orderBy []OrderByField, err *errors.ServiceError) {

	if len(orderByArgs) != 0 {
		orderBy = make([]OrderByField, 0, len(orderByArgs))
		for _, arg := range orderByArgs {
			parts := strings.Fields(arg)
			if len(parts) < 1 || len(parts) > 2 {
				err = errors.BadRequest("an order value is not valid")
				return
			}

			field, fieldErr := getField(parts[0], disallowedFields)
			if fieldErr != nil {
				err = errors.BadRequest("an order value is not valid")
				return
			}

			descending := false
			if len(parts) == 2 {
				switch strings.ToLower(parts[1]) {
				case "asc":
				case "desc":
					descending = true
				default:
					err = errors.BadRequest("an order value is not valid")
					return
				}
			}
			orderBy = append(orderBy, OrderByField{Field: field, Descending: descending})
		}
	}
	return
}

// GetTableName returns the table name associated with the GORM model.
func GetTableName(g2 *gorm.DB) string {
	if g2.Statement.Parse(g2.Statement.Model) != nil {
		return "xxx"
	}
	if g2.Statement.Schema != nil {
		return g2.Statement.Schema.Table
	} else {
		name := reflect.TypeOf(g2.Statement.Model).Elem().Name()
		return inflection.Plural(strings.ToLower(name))
	}
}
