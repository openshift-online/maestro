package handlers

import (
	"reflect"
	"strings"

	"github.com/openshift-online/maestro/pkg/api/openapi"
	"github.com/openshift-online/maestro/pkg/errors"
)

func validateNotEmpty(i interface{}, fieldName string, field string) validate {
	return func() *errors.ServiceError {
		value := reflect.ValueOf(i).Elem().FieldByName(fieldName)
		if value.Kind() == reflect.Ptr {
			if value.IsNil() {
				return errors.Validation("%s is required", field)
			}
			value = value.Elem()
		}
		if len(value.String()) == 0 {
			return errors.Validation("%s is required", field)
		}
		return nil
	}
}

func validateEmpty(i interface{}, fieldName string, field string) validate {
	return func() *errors.ServiceError {
		value := reflect.ValueOf(i).Elem().FieldByName(fieldName)
		if value.Kind() == reflect.Ptr {
			if value.IsNil() {
				return nil
			}
			value = value.Elem()
		}
		if len(value.String()) != 0 {
			return errors.Validation("%s must be empty", field)
		}
		return nil
	}
}

// Note that because this uses strings.EqualFold, it is case-insensitive
func validateInclusionIn(value *string, list []string, category *string) validate {
	return func() *errors.ServiceError {
		for _, item := range list {
			if strings.EqualFold(*value, item) {
				return nil
			}
		}
		if category == nil {
			category = &[]string{"value"}[0]
		}
		return errors.Validation("%s is not a valid %s", *value, *category)
	}
}

func validateResourcePatch(patch *openapi.ResourcePatchRequest) validate {
	return func() *errors.ServiceError {
		return nil
	}
}
