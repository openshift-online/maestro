package presenters

import (
	"fmt"
	"reflect"
	"regexp"
	"strings"
	"time"

	"github.com/openshift-online/maestro/pkg/errors"
)

type ProjectionList struct {
	Kind  string                   `json:"kind"`
	Page  int32                    `json:"page"`
	Size  int32                    `json:"size"`
	Total int32                    `json:"total"`
	Items []map[string]interface{} `json:"items"`
}

/*
	SliceFilter

Convert slice of structures to a []byte stream.
Non-existing fields will cause a validation error

@param fields2Store []string - list of fields to export (from `json` tag)

@param items []interface{} - slice of structures to export

@param kind, page, size, total - from openapi.SubscriptionList et al.

@return []byte
*/
func SliceFilter(fields2Store []string, model interface{}) (*ProjectionList, *errors.ServiceError) {
	if model == nil {
		return nil, errors.Validation("Empty model")
	}

	// Prepare list of required field
	var in = map[string]bool{}
	for i := 0; i < len(fields2Store); i++ {
		in[fields2Store[i]] = true
	}

	reflectValue := reflect.ValueOf(model)
	reflectValue = reflect.Indirect(reflectValue)

	// Initialize result structure
	result := &ProjectionList{
		Kind:  reflectValue.FieldByName("Kind").String(),
		Page:  int32(reflectValue.FieldByName("Page").Int()),
		Size:  int32(reflectValue.FieldByName("Size").Int()),
		Total: int32(reflectValue.FieldByName("Total").Int()),
		Items: nil,
	}

	field := reflectValue.FieldByName("Items").Interface()
	items := reflect.ValueOf(field)
	if items.Len() == 0 {
		return result, nil
	}

	// Validate model
	validateIn := make(map[string]bool)
	for key, value := range in {
		validateIn[key] = value
	}
	if err := validate(items.Index(0).Interface(), validateIn, ""); err != nil {
		return nil, err
	}

	// Convert items
	for i := 0; i < items.Len(); i++ {
		result.Items = append(result.Items, structToMap(items.Index(i).Interface(), in, ""))
	}
	return result, nil
}

func validate(model interface{}, in map[string]bool, prefix string) *errors.ServiceError {
	if model == nil {
		return errors.Validation("Empty model")
	}

	v := reflect.TypeOf(model)
	reflectValue := reflect.ValueOf(model)
	reflectValue = reflect.Indirect(reflectValue)

	if v.Kind() == reflect.Pointer {
		v = v.Elem()
	}

	for i := 0; i < v.NumField(); i++ {
		t := v.Field(i)
		tag := t.Tag.Get("json")
		if tag == "" || tag == "-" {
			continue
		}
		ttype := reflectValue.Field(i)
		kind := ttype.Kind()
		if kind == reflect.Pointer {
			kind = ttype.Elem().Kind()
		}
		field := reflectValue.Field(i).Interface()
		name := strings.Split(tag, ",")[0]
		if kind == reflect.Struct {
			if t.Type == reflect.TypeOf(&time.Time{}) {
				delete(in, name)
			} else {
				star := name + ".*"
				if _, ok := in[star]; ok {
					in = removeStar(in, name)
				} else {
					_ = validate(field, in, name)
				}
			}
		} else if t.Type.Kind() == reflect.Slice {
			// TODO: We don't support Slices' validation :(
			in = removeStar(in, name)
			continue
			//_ = validate(slice, in, name)
		} else {
			prefixedName := name
			if prefix != "" {
				prefixedName = fmt.Sprintf("%s.%s", prefix, name)
			}
			delete(in, prefixedName)
		}
	}

	// All fields present in data struct
	if len(in) == 0 {
		return nil
	}

	var fields []string
	for k := range in {
		fields = append(fields, k)
	}
	message := fmt.Sprintf("The following field(s) doesn't exist in `%s`: %s",
		reflect.TypeOf(model).Name(), strings.Join(fields, ", "))
	return errors.Validation(message)
}

func removeStar(in map[string]bool, name string) map[string]bool {
	pattern := `(` + name + `\..*)`
	pat, _ := regexp.Compile(pattern)
	for k := range in {
		matched := pat.FindAllString(k, -1)
		for _, m := range matched {
			delete(in, m)
		}
	}

	return in
}

func structToMap(item interface{}, in map[string]bool, prefix string) map[string]interface{} {
	res := map[string]interface{}{}

	if item == nil {
		return res
	}
	v := reflect.TypeOf(item)
	reflectValue := reflect.ValueOf(item)
	reflectValue = reflect.Indirect(reflectValue)

	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}

	for i := 0; i < v.NumField(); i++ {
		t := v.Field(i)
		tag := t.Tag.Get("json")
		if tag == "" || tag == "-" {
			continue
		}
		ttype := reflectValue.Field(i)
		kind := ttype.Kind()
		if kind == reflect.Pointer {
			kind = ttype.Elem().Kind()
		}
		field := reflectValue.Field(i).Interface()
		name := strings.Split(tag, ",")[0]
		if kind == reflect.Struct {
			if t.Type == reflect.TypeOf(&time.Time{}) {
				if _, ok := in[name]; ok {
					res[name] = field.(*time.Time).Format(time.RFC3339)
				}
			} else {
				nexPrefix := name
				if prefix != "" {
					nexPrefix = prefix + "." + name
				}
				subStruct := structToMap(field, in, nexPrefix)
				if len(subStruct) > 0 {
					res[name] = subStruct
				}
			}
		} else if kind == reflect.Slice {
			s := reflect.ValueOf(field)
			if s.Len() > 0 {
				result := make([]interface{}, 0, s.Len())
				for i := 0; i < s.Len(); i++ {
					slice := structToMap(s.Index(i).Interface(), in, name)
					if len(slice) == 0 {
						break
					}
					result = append(result, slice)
				}
				if len(result) > 0 {
					res[name] = result
				}
			}
		} else {
			prefixedName := name
			if prefix != "" {
				prefixedName = fmt.Sprintf("%s.%s", prefix, name)
			}
			if _, ok := in[prefixedName]; ok {
				res[name] = field
			} else {
				prefixedStar := fmt.Sprintf("%s.*", prefix)
				if _, ok := in[prefixedStar]; ok {
					res[name] = field
				}
			}
		}
	}

	return res
}
