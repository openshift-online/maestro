package services

import (
	"net/url"
	"strconv"
	"strings"
)

// ListArguments are arguments relevant for listing objects.
// This struct is common to all service List funcs in this package
type ListArguments struct {
	Page     int
	Size     int64
	Preloads []string
	Search   string
	OrderBy  []string
	Fields   []string
}

// ~65500 is the maximum number of parameters that can be provided to a postgres WHERE IN clause
// Use it as a sane max
const MAX_LIST_SIZE = 65500

// Create ListArguments from url query parameters with sane defaults
func NewListArguments(params url.Values) *ListArguments {
	listArgs := &ListArguments{
		Page:   1,
		Size:   100,
		Search: "",
	}
	if v := strings.Trim(params.Get("page"), " "); v != "" {
		listArgs.Page, _ = strconv.Atoi(v)
	}
	if v := strings.Trim(params.Get("size"), " "); v != "" {
		listArgs.Size, _ = strconv.ParseInt(v, 10, 0)
	}
	if listArgs.Size > MAX_LIST_SIZE || listArgs.Size < 0 {
		// MAX_LIST_SIZE is the maximum number of *parameters* that can be provided to a postgres WHERE IN clause
		// Use it as a sane max
		listArgs.Size = MAX_LIST_SIZE
	}
	if v := strings.Trim(params.Get("search"), " "); v != "" {
		listArgs.Search = v
	}
	if v := strings.Trim(params.Get("orderBy"), " "); v != "" {
		listArgs.OrderBy = strings.Split(v, ",")
	}
	if v := strings.Trim(params.Get("fields"), " "); v != "" {
		fields := strings.Split(v, ",")
		idNotPresent := true
		for i := 0; i < len(fields); i++ {
			field := strings.Trim(fields[i], " ")
			if field == "" { // skip leading/trailing commas and spaces
				continue
			}
			if field == "id" {
				idNotPresent = false
			}
			listArgs.Fields = append(listArgs.Fields, field)
		}
		if idNotPresent {
			listArgs.Fields = append(listArgs.Fields, "id")
		}
	}

	return listArgs
}
