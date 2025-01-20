/*
maestro Service API

maestro Service API

API version: 0.0.1
*/

// Code generated by OpenAPI Generator (https://openapi-generator.tech); DO NOT EDIT.

package openapi

import (
	"encoding/json"
)

// checks if the ResourcePatchRequest type satisfies the MappedNullable interface at compile time
var _ MappedNullable = &ResourcePatchRequest{}

// ResourcePatchRequest struct for ResourcePatchRequest
type ResourcePatchRequest struct {
	Version        *int32                 `json:"version,omitempty"`
	Manifest       map[string]interface{} `json:"manifest,omitempty"`
	DeleteOption   map[string]interface{} `json:"delete_option,omitempty"`
	ManifestConfig map[string]interface{} `json:"manifest_config,omitempty"`
}

// NewResourcePatchRequest instantiates a new ResourcePatchRequest object
// This constructor will assign default values to properties that have it defined,
// and makes sure properties required by API are set, but the set of arguments
// will change when the set of required properties is changed
func NewResourcePatchRequest() *ResourcePatchRequest {
	this := ResourcePatchRequest{}
	return &this
}

// NewResourcePatchRequestWithDefaults instantiates a new ResourcePatchRequest object
// This constructor will only assign default values to properties that have it defined,
// but it doesn't guarantee that properties required by API are set
func NewResourcePatchRequestWithDefaults() *ResourcePatchRequest {
	this := ResourcePatchRequest{}
	return &this
}

// GetVersion returns the Version field value if set, zero value otherwise.
func (o *ResourcePatchRequest) GetVersion() int32 {
	if o == nil || IsNil(o.Version) {
		var ret int32
		return ret
	}
	return *o.Version
}

// GetVersionOk returns a tuple with the Version field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *ResourcePatchRequest) GetVersionOk() (*int32, bool) {
	if o == nil || IsNil(o.Version) {
		return nil, false
	}
	return o.Version, true
}

// HasVersion returns a boolean if a field has been set.
func (o *ResourcePatchRequest) HasVersion() bool {
	if o != nil && !IsNil(o.Version) {
		return true
	}

	return false
}

// SetVersion gets a reference to the given int32 and assigns it to the Version field.
func (o *ResourcePatchRequest) SetVersion(v int32) {
	o.Version = &v
}

// GetManifest returns the Manifest field value if set, zero value otherwise.
func (o *ResourcePatchRequest) GetManifest() map[string]interface{} {
	if o == nil || IsNil(o.Manifest) {
		var ret map[string]interface{}
		return ret
	}
	return o.Manifest
}

// GetManifestOk returns a tuple with the Manifest field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *ResourcePatchRequest) GetManifestOk() (map[string]interface{}, bool) {
	if o == nil || IsNil(o.Manifest) {
		return map[string]interface{}{}, false
	}
	return o.Manifest, true
}

// HasManifest returns a boolean if a field has been set.
func (o *ResourcePatchRequest) HasManifest() bool {
	if o != nil && !IsNil(o.Manifest) {
		return true
	}

	return false
}

// SetManifest gets a reference to the given map[string]interface{} and assigns it to the Manifest field.
func (o *ResourcePatchRequest) SetManifest(v map[string]interface{}) {
	o.Manifest = v
}

// GetDeleteOption returns the DeleteOption field value if set, zero value otherwise.
func (o *ResourcePatchRequest) GetDeleteOption() map[string]interface{} {
	if o == nil || IsNil(o.DeleteOption) {
		var ret map[string]interface{}
		return ret
	}
	return o.DeleteOption
}

// GetDeleteOptionOk returns a tuple with the DeleteOption field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *ResourcePatchRequest) GetDeleteOptionOk() (map[string]interface{}, bool) {
	if o == nil || IsNil(o.DeleteOption) {
		return map[string]interface{}{}, false
	}
	return o.DeleteOption, true
}

// HasDeleteOption returns a boolean if a field has been set.
func (o *ResourcePatchRequest) HasDeleteOption() bool {
	if o != nil && !IsNil(o.DeleteOption) {
		return true
	}

	return false
}

// SetDeleteOption gets a reference to the given map[string]interface{} and assigns it to the DeleteOption field.
func (o *ResourcePatchRequest) SetDeleteOption(v map[string]interface{}) {
	o.DeleteOption = v
}

// GetManifestConfig returns the ManifestConfig field value if set, zero value otherwise.
func (o *ResourcePatchRequest) GetManifestConfig() map[string]interface{} {
	if o == nil || IsNil(o.ManifestConfig) {
		var ret map[string]interface{}
		return ret
	}
	return o.ManifestConfig
}

// GetManifestConfigOk returns a tuple with the ManifestConfig field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *ResourcePatchRequest) GetManifestConfigOk() (map[string]interface{}, bool) {
	if o == nil || IsNil(o.ManifestConfig) {
		return map[string]interface{}{}, false
	}
	return o.ManifestConfig, true
}

// HasManifestConfig returns a boolean if a field has been set.
func (o *ResourcePatchRequest) HasManifestConfig() bool {
	if o != nil && !IsNil(o.ManifestConfig) {
		return true
	}

	return false
}

// SetManifestConfig gets a reference to the given map[string]interface{} and assigns it to the ManifestConfig field.
func (o *ResourcePatchRequest) SetManifestConfig(v map[string]interface{}) {
	o.ManifestConfig = v
}

func (o ResourcePatchRequest) MarshalJSON() ([]byte, error) {
	toSerialize, err := o.ToMap()
	if err != nil {
		return []byte{}, err
	}
	return json.Marshal(toSerialize)
}

func (o ResourcePatchRequest) ToMap() (map[string]interface{}, error) {
	toSerialize := map[string]interface{}{}
	if !IsNil(o.Version) {
		toSerialize["version"] = o.Version
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
	return toSerialize, nil
}

type NullableResourcePatchRequest struct {
	value *ResourcePatchRequest
	isSet bool
}

func (v NullableResourcePatchRequest) Get() *ResourcePatchRequest {
	return v.value
}

func (v *NullableResourcePatchRequest) Set(val *ResourcePatchRequest) {
	v.value = val
	v.isSet = true
}

func (v NullableResourcePatchRequest) IsSet() bool {
	return v.isSet
}

func (v *NullableResourcePatchRequest) Unset() {
	v.value = nil
	v.isSet = false
}

func NewNullableResourcePatchRequest(val *ResourcePatchRequest) *NullableResourcePatchRequest {
	return &NullableResourcePatchRequest{value: val, isSet: true}
}

func (v NullableResourcePatchRequest) MarshalJSON() ([]byte, error) {
	return json.Marshal(v.value)
}

func (v *NullableResourcePatchRequest) UnmarshalJSON(src []byte) error {
	v.isSet = true
	return json.Unmarshal(src, &v.value)
}
