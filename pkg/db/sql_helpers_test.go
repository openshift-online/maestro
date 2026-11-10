package db

import (
	"testing"

	maestroerrors "github.com/openshift-online/maestro/pkg/errors"
)

func TestGetFieldRejectsSQLExpressions(t *testing.T) {
	tests := []struct {
		name  string
		valid bool
	}{
		{name: "name", valid: true},
		{name: "resources.created_at", valid: true},
		{name: "properties ->> 'name'", valid: false},
		{name: "name' OR 1=1 --", valid: false},
		{name: `"name' OR 1=1 --"`, valid: false},
		{name: "name;DROP", valid: false},
		{name: "name()", valid: false},
		{name: "resources.name.extra", valid: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			field, serviceErr := getField(test.name, nil)
			if test.valid {
				if serviceErr != nil {
					t.Fatalf("expected valid identifier, got error: %v", serviceErr)
				}
				if field != test.name {
					t.Fatalf("expected field %q, got %q", test.name, field)
				}
				return
			}
			if serviceErr == nil || serviceErr.Code != maestroerrors.ErrorBadRequest {
				t.Fatalf("expected bad request for %q, got field=%q error=%v", test.name, field, serviceErr)
			}
		})
	}
}

func TestArgsToOrderBySeparatesFieldsAndDirections(t *testing.T) {
	got, serviceErr := ArgsToOrderBy([]string{"name desc", "created_at ASC"}, nil)
	if serviceErr != nil {
		t.Fatalf("expected valid ordering, got error: %v", serviceErr)
	}
	if len(got) != 2 || got[0] != (OrderByField{Field: "name", Descending: true}) || got[1] != (OrderByField{Field: "created_at"}) {
		t.Fatalf("unexpected parsed ordering: %#v", got)
	}
}

func TestArgsToOrderByRejectsSQLExpressions(t *testing.T) {
	for _, arg := range []string{
		"name desc; DROP TABLE resources",
		`name"; DROP TABLE resources --`,
		"name desc -- comment",
		"name;",
		"lower(name)",
		"name sideways",
	} {
		t.Run(arg, func(t *testing.T) {
			_, serviceErr := ArgsToOrderBy([]string{arg}, nil)
			if serviceErr == nil || serviceErr.Code != maestroerrors.ErrorBadRequest {
				t.Fatalf("expected bad request for %q, got %v", arg, serviceErr)
			}
		})
	}
}
