package handlers

import (
	"reflect"

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
