/*
maestro Service API

maestro Service API

API version: 0.0.1
*/

// Code generated by OpenAPI Generator (https://openapi-generator.tech); DO NOT EDIT.

package openapi

import (
	"encoding/json"
	"time"
)

// checks if the ResourceAllOf type satisfies the MappedNullable interface at compile time
var _ MappedNullable = &ResourceAllOf{}

// ResourceAllOf struct for ResourceAllOf
type ResourceAllOf struct {
	Name           *string                `json:"name,omitempty"`
	ConsumerName   *string                `json:"consumer_name,omitempty"`
	Version        *int32                 `json:"version,omitempty"`
	CreatedAt      *time.Time             `json:"created_at,omitempty"`
	UpdatedAt      *time.Time             `json:"updated_at,omitempty"`
	DeletedAt      *time.Time             `json:"deleted_at,omitempty"`
	Manifest       map[string]interface{} `json:"manifest,omitempty"`
	DeleteOption   map[string]interface{} `json:"delete_option,omitempty"`
	ManifestConfig map[string]interface{} `json:"manifest_config,omitempty"`
	Status         map[string]interface{} `json:"status,omitempty"`
}

// NewResourceAllOf instantiates a new ResourceAllOf object
// This constructor will assign default values to properties that have it defined,
// and makes sure properties required by API are set, but the set of arguments
// will change when the set of required properties is changed
func NewResourceAllOf() *ResourceAllOf {
	this := ResourceAllOf{}
	return &this
}

// NewResourceAllOfWithDefaults instantiates a new ResourceAllOf object
// This constructor will only assign default values to properties that have it defined,
// but it doesn't guarantee that properties required by API are set
func NewResourceAllOfWithDefaults() *ResourceAllOf {
	this := ResourceAllOf{}
	return &this
}

// GetName returns the Name field value if set, zero value otherwise.
func (o *ResourceAllOf) GetName() string {
	if o == nil || IsNil(o.Name) {
		var ret string
		return ret
	}
	return *o.Name
}

// GetNameOk returns a tuple with the Name field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *ResourceAllOf) GetNameOk() (*string, bool) {
	if o == nil || IsNil(o.Name) {
		return nil, false
	}
	return o.Name, true
}

// HasName returns a boolean if a field has been set.
func (o *ResourceAllOf) HasName() bool {
	if o != nil && !IsNil(o.Name) {
		return true
	}

	return false
}

// SetName gets a reference to the given string and assigns it to the Name field.
func (o *ResourceAllOf) SetName(v string) {
	o.Name = &v
}

// GetConsumerName returns the ConsumerName field value if set, zero value otherwise.
func (o *ResourceAllOf) GetConsumerName() string {
	if o == nil || IsNil(o.ConsumerName) {
		var ret string
		return ret
	}
	return *o.ConsumerName
}

// GetConsumerNameOk returns a tuple with the ConsumerName field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *ResourceAllOf) GetConsumerNameOk() (*string, bool) {
	if o == nil || IsNil(o.ConsumerName) {
		return nil, false
	}
	return o.ConsumerName, true
}

// HasConsumerName returns a boolean if a field has been set.
func (o *ResourceAllOf) HasConsumerName() bool {
	if o != nil && !IsNil(o.ConsumerName) {
		return true
	}

	return false
}

// SetConsumerName gets a reference to the given string and assigns it to the ConsumerName field.
func (o *ResourceAllOf) SetConsumerName(v string) {
	o.ConsumerName = &v
}

// GetVersion returns the Version field value if set, zero value otherwise.
func (o *ResourceAllOf) GetVersion() int32 {
	if o == nil || IsNil(o.Version) {
		var ret int32
		return ret
	}
	return *o.Version
}

// GetVersionOk returns a tuple with the Version field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *ResourceAllOf) GetVersionOk() (*int32, bool) {
	if o == nil || IsNil(o.Version) {
		return nil, false
	}
	return o.Version, true
}

// HasVersion returns a boolean if a field has been set.
func (o *ResourceAllOf) HasVersion() bool {
	if o != nil && !IsNil(o.Version) {
		return true
	}

	return false
}

// SetVersion gets a reference to the given int32 and assigns it to the Version field.
func (o *ResourceAllOf) SetVersion(v int32) {
	o.Version = &v
}

// GetCreatedAt returns the CreatedAt field value if set, zero value otherwise.
func (o *ResourceAllOf) GetCreatedAt() time.Time {
	if o == nil || IsNil(o.CreatedAt) {
		var ret time.Time
		return ret
	}
	return *o.CreatedAt
}

// GetCreatedAtOk returns a tuple with the CreatedAt field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *ResourceAllOf) GetCreatedAtOk() (*time.Time, bool) {
	if o == nil || IsNil(o.CreatedAt) {
		return nil, false
	}
	return o.CreatedAt, true
}

// HasCreatedAt returns a boolean if a field has been set.
func (o *ResourceAllOf) HasCreatedAt() bool {
	if o != nil && !IsNil(o.CreatedAt) {
		return true
	}

	return false
}

// SetCreatedAt gets a reference to the given time.Time and assigns it to the CreatedAt field.
func (o *ResourceAllOf) SetCreatedAt(v time.Time) {
	o.CreatedAt = &v
}

// GetUpdatedAt returns the UpdatedAt field value if set, zero value otherwise.
func (o *ResourceAllOf) GetUpdatedAt() time.Time {
	if o == nil || IsNil(o.UpdatedAt) {
		var ret time.Time
		return ret
	}
	return *o.UpdatedAt
}

// GetUpdatedAtOk returns a tuple with the UpdatedAt field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *ResourceAllOf) GetUpdatedAtOk() (*time.Time, bool) {
	if o == nil || IsNil(o.UpdatedAt) {
		return nil, false
	}
	return o.UpdatedAt, true
}

// HasUpdatedAt returns a boolean if a field has been set.
func (o *ResourceAllOf) HasUpdatedAt() bool {
	if o != nil && !IsNil(o.UpdatedAt) {
		return true
	}

	return false
}

// SetUpdatedAt gets a reference to the given time.Time and assigns it to the UpdatedAt field.
func (o *ResourceAllOf) SetUpdatedAt(v time.Time) {
	o.UpdatedAt = &v
}

// GetDeletedAt returns the DeletedAt field value if set, zero value otherwise.
func (o *ResourceAllOf) GetDeletedAt() time.Time {
	if o == nil || IsNil(o.DeletedAt) {
		var ret time.Time
		return ret
	}
	return *o.DeletedAt
}

// GetDeletedAtOk returns a tuple with the DeletedAt field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *ResourceAllOf) GetDeletedAtOk() (*time.Time, bool) {
	if o == nil || IsNil(o.DeletedAt) {
		return nil, false
	}
	return o.DeletedAt, true
}

// HasDeletedAt returns a boolean if a field has been set.
func (o *ResourceAllOf) HasDeletedAt() bool {
	if o != nil && !IsNil(o.DeletedAt) {
		return true
	}

	return false
}

// SetDeletedAt gets a reference to the given time.Time and assigns it to the DeletedAt field.
func (o *ResourceAllOf) SetDeletedAt(v time.Time) {
	o.DeletedAt = &v
}

// GetManifest returns the Manifest field value if set, zero value otherwise.
func (o *ResourceAllOf) GetManifest() map[string]interface{} {
	if o == nil || IsNil(o.Manifest) {
		var ret map[string]interface{}
		return ret
	}
	return o.Manifest
}

// GetManifestOk returns a tuple with the Manifest field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *ResourceAllOf) GetManifestOk() (map[string]interface{}, bool) {
	if o == nil || IsNil(o.Manifest) {
		return map[string]interface{}{}, false
	}
	return o.Manifest, true
}

// HasManifest returns a boolean if a field has been set.
func (o *ResourceAllOf) HasManifest() bool {
	if o != nil && !IsNil(o.Manifest) {
		return true
	}

	return false
}

// SetManifest gets a reference to the given map[string]interface{} and assigns it to the Manifest field.
func (o *ResourceAllOf) SetManifest(v map[string]interface{}) {
	o.Manifest = v
}

// GetDeleteOption returns the DeleteOption field value if set, zero value otherwise.
func (o *ResourceAllOf) GetDeleteOption() map[string]interface{} {
	if o == nil || IsNil(o.DeleteOption) {
		var ret map[string]interface{}
		return ret
	}
	return o.DeleteOption
}

// GetDeleteOptionOk returns a tuple with the DeleteOption field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *ResourceAllOf) GetDeleteOptionOk() (map[string]interface{}, bool) {
	if o == nil || IsNil(o.DeleteOption) {
		return map[string]interface{}{}, false
	}
	return o.DeleteOption, true
}

// HasDeleteOption returns a boolean if a field has been set.
func (o *ResourceAllOf) HasDeleteOption() bool {
	if o != nil && !IsNil(o.DeleteOption) {
		return true
	}

	return false
}

// SetDeleteOption gets a reference to the given map[string]interface{} and assigns it to the DeleteOption field.
func (o *ResourceAllOf) SetDeleteOption(v map[string]interface{}) {
	o.DeleteOption = v
}

// GetManifestConfig returns the ManifestConfig field value if set, zero value otherwise.
func (o *ResourceAllOf) GetManifestConfig() map[string]interface{} {
	if o == nil || IsNil(o.ManifestConfig) {
		var ret map[string]interface{}
		return ret
	}
	return o.ManifestConfig
}

// GetManifestConfigOk returns a tuple with the ManifestConfig field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *ResourceAllOf) GetManifestConfigOk() (map[string]interface{}, bool) {
	if o == nil || IsNil(o.ManifestConfig) {
		return map[string]interface{}{}, false
	}
	return o.ManifestConfig, true
}

// HasManifestConfig returns a boolean if a field has been set.
func (o *ResourceAllOf) HasManifestConfig() bool {
	if o != nil && !IsNil(o.ManifestConfig) {
		return true
	}

	return false
}

// SetManifestConfig gets a reference to the given map[string]interface{} and assigns it to the ManifestConfig field.
func (o *ResourceAllOf) SetManifestConfig(v map[string]interface{}) {
	o.ManifestConfig = v
}

// GetStatus returns the Status field value if set, zero value otherwise.
func (o *ResourceAllOf) GetStatus() map[string]interface{} {
	if o == nil || IsNil(o.Status) {
		var ret map[string]interface{}
		return ret
	}
	return o.Status
}

// GetStatusOk returns a tuple with the Status field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *ResourceAllOf) GetStatusOk() (map[string]interface{}, bool) {
	if o == nil || IsNil(o.Status) {
		return map[string]interface{}{}, false
	}
	return o.Status, true
}

// HasStatus returns a boolean if a field has been set.
func (o *ResourceAllOf) HasStatus() bool {
	if o != nil && !IsNil(o.Status) {
		return true
	}

	return false
}

// SetStatus gets a reference to the given map[string]interface{} and assigns it to the Status field.
func (o *ResourceAllOf) SetStatus(v map[string]interface{}) {
	o.Status = v
}

func (o ResourceAllOf) MarshalJSON() ([]byte, error) {
	toSerialize, err := o.ToMap()
	if err != nil {
		return []byte{}, err
	}
	return json.Marshal(toSerialize)
}

func (o ResourceAllOf) ToMap() (map[string]interface{}, error) {
	toSerialize := map[string]interface{}{}
	if !IsNil(o.Name) {
		toSerialize["name"] = o.Name
	}
	if !IsNil(o.ConsumerName) {
		toSerialize["consumer_name"] = o.ConsumerName
	}
	if !IsNil(o.Version) {
		toSerialize["version"] = o.Version
	}
	if !IsNil(o.CreatedAt) {
		toSerialize["created_at"] = o.CreatedAt
	}
	if !IsNil(o.UpdatedAt) {
		toSerialize["updated_at"] = o.UpdatedAt
	}
	if !IsNil(o.DeletedAt) {
		toSerialize["deleted_at"] = o.DeletedAt
	}
	if !IsNil(o.Manifest) {
		toSerialize["manifest"] = o.Manifest
	}
	if !IsNil(o.DeleteOption) {
		toSerialize["delete_option"] = o.DeleteOption
	}
	if !IsNil(o.ManifestConfig) {
		toSerialize["manifest_config"] = o.ManifestConfig
	}
	if !IsNil(o.Status) {
		toSerialize["status"] = o.Status
	}
	return toSerialize, nil
}

type NullableResourceAllOf struct {
	value *ResourceAllOf
	isSet bool
}

func (v NullableResourceAllOf) Get() *ResourceAllOf {
	return v.value
}

func (v *NullableResourceAllOf) Set(val *ResourceAllOf) {
	v.value = val
	v.isSet = true
}

func (v NullableResourceAllOf) IsSet() bool {
	return v.isSet
}

func (v *NullableResourceAllOf) Unset() {
	v.value = nil
	v.isSet = false
}

func NewNullableResourceAllOf(val *ResourceAllOf) *NullableResourceAllOf {
	return &NullableResourceAllOf{value: val, isSet: true}
}

func (v NullableResourceAllOf) MarshalJSON() ([]byte, error) {
	return json.Marshal(v.value)
}

func (v *NullableResourceAllOf) UnmarshalJSON(src []byte) error {
	v.isSet = true
	return json.Unmarshal(src, &v.value)
}
