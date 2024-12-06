package handlers

import (
	"reflect"

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

// validateDeleteOptionAndUpdateStrategy validates the delete option and update strategy
// for a resource, to ensure that update strategy ReadOnly is only allowed with delete option Orphan.
func validateDeleteOptionAndUpdateStrategy(rs *openapi.Resource) validate {
	return func() *errors.ServiceError {
		if rs.DeleteOption != nil && rs.UpdateStrategy != nil {
			deleteType, ok := rs.DeleteOption["propagationPolicy"].(string)
			if !ok {
				return errors.Validation("invalid delete option")
			}
			updateStrategy, ok := rs.UpdateStrategy["type"].(string)
			if !ok {
				return errors.Validation("invalid update strategy")
			}
			if deleteType != "Orphan" && updateStrategy == "ReadOnly" {
				return errors.Validation("update strategy ReadOnly is only allowed with delete option Orphan")
			}
		}
		return nil
	}
}
