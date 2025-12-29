package db

import (
	"database/sql/driver"
	"encoding/json"
)

// similar to gorms datatypes.JSONMap but it restricts the values to strings
type StringMap map[string]string

func (m *StringMap) Scan(value interface{}) error {
	return json.Unmarshal(value.([]byte), m)
}

func (m StringMap) Value() (driver.Value, error) {
	return json.Marshal(m)
}

func (m *StringMap) ToMap() *map[string]string {
	if m == nil {
		return nil
	}
	return (*map[string]string)(m)
}

func EmptyMapToNilStringMap(a *map[string]string) *StringMap {
	if a == nil {
		return nil
	}
	if len(*a) == 0 {
		return nil
	}
	sm := StringMap(*a)
	return &sm
}
