package db

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/jinzhu/inflection"
	"github.com/yaacov/tree-search-language/pkg/tsl"
	"gorm.io/gorm"

	"github.com/openshift-online/maestro/pkg/errors"
)

// Check if a field name starts with properties.
func startsWithProperties(s string) bool {
	return strings.HasPrefix(s, "properties.")
}

// hasProperty return true if node has a property identifier on left hand side.
func hasProperty(n tsl.Node) bool {
	// Get the left side operator.
	l, ok := n.Left.(tsl.Node)
	if !ok {
		return false
	}

	// If left side hand is not a `properties` identifier, return.
	if l.Func != tsl.IdentOp || !startsWithProperties(l.Left.(string)) {
		return false
	}

	return true
}

// getField gets the sql field associated with a name.
func getField(name string, disallowedFields map[string]string) (field string, err *errors.ServiceError) {
	// We want to accept names with trailing and leading spaces
	trimmedName := strings.Trim(name, " ")

	// Check for properties ->> '<some field name>'
	if strings.HasPrefix(trimmedName, "properties ->>") {
		field = trimmedName
		return
	}

	// Check for nested field, e.g., subscription_labels.key
	checkName := trimmedName
	fieldParts := strings.Split(trimmedName, ".")
	if len(fieldParts) > 2 {
		err = errors.BadRequest("%s is not a valid field name", name)
		return
	}
	if len(fieldParts) > 1 {
		checkName = fieldParts[1]
	}

	// Check for allowed fields
	_, ok := disallowedFields[checkName]
	if ok {
		err = errors.BadRequest("%s is not a valid field name", name)
		return
	}
	field = trimmedName
	return
}

// propertiesNodeConverter converts a node with a properties identifier
// to a properties node.
//
// For example, it will convert:
// ( properties.<name> = <value> ) to
// ( properties ->> <name> = <value> )
func propertiesNodeConverter(n tsl.Node) tsl.Node {

	// Get the left side operator.
	l, ok := n.Left.(tsl.Node)
	if !ok {
		return n
	}

	// Get the property name.
	propetyName := l.Left.(string)[11:]

	// Build a new node that converts:
	// ( properties.<name> = <value> ) to
	// ( properties ->> <name> = <value> )
	propertyNode := tsl.Node{
		Func: n.Func,
		Left: tsl.Node{
			Func: tsl.IdentOp,
			Left: fmt.Sprintf("properties ->> '%s'", propetyName),
		},
		Right: n.Right,
	}

	return propertyNode
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

	// Check for properties.<name> = <value> nodes, and convert them to
	// ( properties ->> <name> = <value> )
	// nodes.
	if hasProperty(n) {
		n = propertiesNodeConverter(n)
	}

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

// cleanOrderBy takes the orderBy arg and cleans it.
func cleanOrderBy(userArg string, disallowedFields map[string]string) (orderBy string, err *errors.ServiceError) {
	var orderField string

	// We want to accept user params with trailing and leading spaces
	trimedName := strings.Trim(userArg, " ")

	// Each OrderBy can be a "<field-name>" or a "<field-name> asc|desc"
	order := strings.Split(trimedName, " ")
	direction := "none valid"

	if len(order) == 1 {
		orderField, err = getField(order[0], disallowedFields)
		direction = "asc"
	} else if len(order) == 2 {
		orderField, err = getField(order[0], disallowedFields)
		direction = order[1]
	}
	if err != nil || (direction != "asc" && direction != "desc") {
		err = errors.BadRequest("bad order value '%s'", userArg)
		return
	}

	orderBy = fmt.Sprintf("%s %s", orderField, direction)
	return
}

// ArgsToOrderBy returns cleaned orderBy list.
func ArgsToOrderBy(
	orderByArgs []string,
	disallowedFields map[string]string) (orderBy []string, err *errors.ServiceError) {

	var order string
	if len(orderByArgs) != 0 {
		orderBy = []string{}
		for _, o := range orderByArgs {
			order, err = cleanOrderBy(o, disallowedFields)
			if err != nil {
				return
			}

			// If valid add the user entered order by, to the order by list
			orderBy = append(orderBy, order)
		}
	}
	return
}

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
