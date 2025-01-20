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

func validateManifestConfig(rs *openapi.Resource) validate {
	return func() *errors.ServiceError {
		if rs.ManifestConfig == nil {
			return errors.Validation("manifest config is required")
		}
		resourceIdentifier, ok := rs.ManifestConfig["resourceIdentifier"]
		if !ok {
			return errors.Validation("resource identifier is required")
		}
		resourceIdentifierMap, ok := resourceIdentifier.(map[string]interface{})
		if !ok {
			return errors.Validation("resource identifier must be a map")
		}

		if group, ok := resourceIdentifierMap["group"]; !ok {
			return errors.Validation("resource identifier group is required")
		} else {
			// group is required to be a string, but may be empty string
			if _, ok := group.(string); !ok {
				return errors.Validation("resource identifier group must be a string")
			}
		}
		if resource, ok := resourceIdentifierMap["resource"]; !ok {
			return errors.Validation("resource identifier resource is required")
		} else {
			if resourceVal, ok := resource.(string); !ok || len(resourceVal) == 0 {
				return errors.Validation("resource identifier resource must be a non-empty string")
			}
		}
		if name, ok := resourceIdentifierMap["name"]; !ok {
			return errors.Validation("resource identifier name is required")
		} else {
			if nameVal, ok := name.(string); !ok || len(nameVal) == 0 {
				return errors.Validation("resource identifier name must be a non-empty string")
			}
		}
		if namespace, ok := resourceIdentifierMap["namespace"]; !ok {
			return errors.Validation("resource identifier namespace is required")
		} else {
			if namespaceVal, ok := namespace.(string); !ok || len(namespaceVal) == 0 {
				return errors.Validation("resource identifier namespace must be a non-empty string")
			}
		}
		return nil
	}
}

// validateDeleteOptionAndUpdateStrategy validates the delete option and update strategy
// for a resource, to ensure that update strategy ReadOnly is only allowed with delete option Orphan.
func validateDeleteOptionAndUpdateStrategy(rs *openapi.Resource) validate {
	return func() *errors.ServiceError {
		if rs.DeleteOption != nil && rs.ManifestConfig != nil {
			deleteType, ok := rs.DeleteOption["propagationPolicy"].(string)
			if !ok {
				return errors.Validation("invalid delete option")
			}
			updateStrategy, ok := rs.ManifestConfig["updateStrategy"]
			if ok {
				updateStrategyVal, ok := updateStrategy.(map[string]interface{})
				if !ok {
					return errors.Validation("invalid update strategy")
				}
				updateStrategy, ok := updateStrategyVal["type"].(string)
				if !ok {
					return errors.Validation("invalid update strategy type")
				}
				if deleteType != "Orphan" && updateStrategy == "ReadOnly" {
					return errors.Validation("update strategy ReadOnly is only allowed with delete option Orphan")
				}
			}
		}
		return nil
	}
}
