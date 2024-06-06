// copy code from https://github.com/yaacov/tree-search-language/blob/master/v5/pkg/walkers/ident/walk.go
// enhance it to support jsonb object query - https://www.postgresql.org/docs/9.4/functions-json.html
// Package ident helps to replace identifiers in a TSL tree.
package ident

import (
	"github.com/yaacov/tree-search-language/v5/pkg/tsl"
)

// Walk travel the TSL tree and replace identifiers.
//
// Users can call the Walk method to check and replace identifiers.
//
// Example:
//
//	columnNamesMap :=  map[string]string{
//		"title":       "title",
//		"author":      "author",
//		"spec.pages":  "pages",
//		"spec.rating": "rating",
//	}
//
//	func check(s string) (string, error) {
//		// Chekc for column name in map.
//		if v, ok := columnNamesMap[s]; ok {
//			return v, nil
//		}
//
//		// If not found return string as is, and an error.
//		return s, fmt.Errorf("column not found")
//	}
//
//	// Check and replace user identifiers with the SQL table column names.
//	//
//	// SQL table columns are "title, author, pages and rating", but for
//	// users pages and ratings are fields of an internal struct called
//	// spec (e.g. {"title": "Book", "author": "Joe", "spec": {"pages": 5, "rating": 5}}).
//	//
//	newTree, err = ident.Walk(tree, check)
var ignoreRight bool

func Walk(n tsl.Node, siblingN tsl.Node, checkColumnName func(interface{}, tsl.Node) (string, bool, error)) (tsl.Node, error) {
	var err error
	var v string

	// Walk tree.
	switch n.Func {
	case tsl.IdentOp:
		// If we have an identifier, check for it in the identMap.
		v, ignoreRight, err = checkColumnName(n.Left, siblingN)
		if err == nil {
			// If valid identifier, use it.
			n.Left = v
			return n, nil
		}

		return n, err
	case tsl.StringOp, tsl.NumberOp, tsl.BooleanOp, tsl.DateOp:
		// This are our leafs.
		ignoreRight = false
		return n, nil
	default:
		ignoreRight = false
		// Check identifiers on left side.
		if n.Left != nil {
			siblingN := tsl.Node{}
			if n.Right != nil {
				siblingN = n.Right.(tsl.Node)
			}
			n.Left, err = Walk(n.Left.(tsl.Node), siblingN, checkColumnName)
			if err != nil {
				return n, err
			}
		}

		if ignoreRight {
			n.Right = tsl.Node{}
			n.Func = tsl.NullOp
			return n, nil
		}

		// Check identifiers on right side.
		if n.Right != nil {
			// Check if right hand arg is a node, or an array of nodes.
			//
			// If it's an array of nodes.
			// We assume that all are leafs, no nead to walk on them.
			if n.Func != tsl.ArrayOp {
				// It's a node.
				n.Right, err = Walk(n.Right.(tsl.Node), tsl.Node{}, checkColumnName)
				if err != nil {
					return n, err
				}
			}
		}

		return n, nil
	}
}
