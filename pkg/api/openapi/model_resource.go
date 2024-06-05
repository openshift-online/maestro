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

// checks if the Resource type satisfies the MappedNullable interface at compile time
var _ MappedNullable = &Resource{}

// Resource struct for Resource
type Resource struct {
	Id             *string                `json:"id,omitempty"`
	Kind           *string                `json:"kind,omitempty"`
	Href           *string                `json:"href,omitempty"`
	Name           *string                `json:"name,omitempty"`
	ConsumerName   *string                `json:"consumer_name,omitempty"`
	Version        *int32                 `json:"version,omitempty"`
	CreatedAt      *time.Time             `json:"created_at,omitempty"`
	UpdatedAt      *time.Time             `json:"updated_at,omitempty"`
	DeletedAt      *time.Time             `json:"deleted_at,omitempty"`
	Manifest       map[string]interface{} `json:"manifest,omitempty"`
	DeleteOption   map[string]interface{} `json:"delete_option,omitempty"`
	UpdateStrategy map[string]interface{} `json:"update_strategy,omitempty"`
	Status         map[string]interface{} `json:"status,omitempty"`
}

// NewResource instantiates a new Resource object
// This constructor will assign default values to properties that have it defined,
// and makes sure properties required by API are set, but the set of arguments
// will change when the set of required properties is changed
func NewResource() *Resource {
	this := Resource{}
	return &this
}

// NewResourceWithDefaults instantiates a new Resource object
// This constructor will only assign default values to properties that have it defined,
// but it doesn't guarantee that properties required by API are set
func NewResourceWithDefaults() *Resource {
	this := Resource{}
	return &this
}

// GetId returns the Id field value if set, zero value otherwise.
func (o *Resource) GetId() string {
	if o == nil || IsNil(o.Id) {
		var ret string
		return ret
	}
	return *o.Id
}

// GetIdOk returns a tuple with the Id field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *Resource) GetIdOk() (*string, bool) {
	if o == nil || IsNil(o.Id) {
		return nil, false
	}
	return o.Id, true
}

// HasId returns a boolean if a field has been set.
func (o *Resource) HasId() bool {
	if o != nil && !IsNil(o.Id) {
		return true
	}

	return false
}

// SetId gets a reference to the given string and assigns it to the Id field.
func (o *Resource) SetId(v string) {
	o.Id = &v
}

// GetKind returns the Kind field value if set, zero value otherwise.
func (o *Resource) GetKind() string {
	if o == nil || IsNil(o.Kind) {
		var ret string
		return ret
	}
	return *o.Kind
}

// GetKindOk returns a tuple with the Kind field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *Resource) GetKindOk() (*string, bool) {
	if o == nil || IsNil(o.Kind) {
		return nil, false
	}
	return o.Kind, true
}

// HasKind returns a boolean if a field has been set.
func (o *Resource) HasKind() bool {
	if o != nil && !IsNil(o.Kind) {
		return true
	}

	return false
}

// SetKind gets a reference to the given string and assigns it to the Kind field.
func (o *Resource) SetKind(v string) {
	o.Kind = &v
}

// GetHref returns the Href field value if set, zero value otherwise.
func (o *Resource) GetHref() string {
	if o == nil || IsNil(o.Href) {
		var ret string
		return ret
	}
	return *o.Href
}

// GetHrefOk returns a tuple with the Href field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *Resource) GetHrefOk() (*string, bool) {
	if o == nil || IsNil(o.Href) {
		return nil, false
	}
	return o.Href, true
}

// HasHref returns a boolean if a field has been set.
func (o *Resource) HasHref() bool {
	if o != nil && !IsNil(o.Href) {
		return true
	}

	return false
}

// SetHref gets a reference to the given string and assigns it to the Href field.
func (o *Resource) SetHref(v string) {
	o.Href = &v
}

// GetName returns the Name field value if set, zero value otherwise.
func (o *Resource) GetName() string {
	if o == nil || IsNil(o.Name) {
		var ret string
		return ret
	}
	return *o.Name
}

// GetNameOk returns a tuple with the Name field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *Resource) GetNameOk() (*string, bool) {
	if o == nil || IsNil(o.Name) {
		return nil, false
	}
	return o.Name, true
}

// HasName returns a boolean if a field has been set.
func (o *Resource) HasName() bool {
	if o != nil && !IsNil(o.Name) {
		return true
	}

	return false
}

// SetName gets a reference to the given string and assigns it to the Name field.
func (o *Resource) SetName(v string) {
	o.Name = &v
}

// GetConsumerName returns the ConsumerName field value if set, zero value otherwise.
func (o *Resource) GetConsumerName() string {
	if o == nil || IsNil(o.ConsumerName) {
		var ret string
		return ret
	}
	return *o.ConsumerName
}

// GetConsumerNameOk returns a tuple with the ConsumerName field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *Resource) GetConsumerNameOk() (*string, bool) {
	if o == nil || IsNil(o.ConsumerName) {
		return nil, false
	}
	return o.ConsumerName, true
}

// HasConsumerName returns a boolean if a field has been set.
func (o *Resource) HasConsumerName() bool {
	if o != nil && !IsNil(o.ConsumerName) {
		return true
	}

	return false
}

// SetConsumerName gets a reference to the given string and assigns it to the ConsumerName field.
func (o *Resource) SetConsumerName(v string) {
	o.ConsumerName = &v
}

// GetVersion returns the Version field value if set, zero value otherwise.
func (o *Resource) GetVersion() int32 {
	if o == nil || IsNil(o.Version) {
		var ret int32
		return ret
	}
	return *o.Version
}

// GetVersionOk returns a tuple with the Version field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *Resource) GetVersionOk() (*int32, bool) {
	if o == nil || IsNil(o.Version) {
		return nil, false
	}
	return o.Version, true
}

// HasVersion returns a boolean if a field has been set.
func (o *Resource) HasVersion() bool {
	if o != nil && !IsNil(o.Version) {
		return true
	}

	return false
}

// SetVersion gets a reference to the given int32 and assigns it to the Version field.
func (o *Resource) SetVersion(v int32) {
	o.Version = &v
}

// GetCreatedAt returns the CreatedAt field value if set, zero value otherwise.
func (o *Resource) GetCreatedAt() time.Time {
	if o == nil || IsNil(o.CreatedAt) {
		var ret time.Time
		return ret
	}
	return *o.CreatedAt
}

// GetCreatedAtOk returns a tuple with the CreatedAt field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *Resource) GetCreatedAtOk() (*time.Time, bool) {
	if o == nil || IsNil(o.CreatedAt) {
		return nil, false
	}
	return o.CreatedAt, true
}

// HasCreatedAt returns a boolean if a field has been set.
func (o *Resource) HasCreatedAt() bool {
	if o != nil && !IsNil(o.CreatedAt) {
		return true
	}

	return false
}

// SetCreatedAt gets a reference to the given time.Time and assigns it to the CreatedAt field.
func (o *Resource) SetCreatedAt(v time.Time) {
	o.CreatedAt = &v
}

// GetUpdatedAt returns the UpdatedAt field value if set, zero value otherwise.
func (o *Resource) GetUpdatedAt() time.Time {
	if o == nil || IsNil(o.UpdatedAt) {
		var ret time.Time
		return ret
	}
	return *o.UpdatedAt
}

// GetUpdatedAtOk returns a tuple with the UpdatedAt field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *Resource) GetUpdatedAtOk() (*time.Time, bool) {
	if o == nil || IsNil(o.UpdatedAt) {
		return nil, false
	}
	return o.UpdatedAt, true
}

// HasUpdatedAt returns a boolean if a field has been set.
func (o *Resource) HasUpdatedAt() bool {
	if o != nil && !IsNil(o.UpdatedAt) {
		return true
	}

	return false
}

// SetUpdatedAt gets a reference to the given time.Time and assigns it to the UpdatedAt field.
func (o *Resource) SetUpdatedAt(v time.Time) {
	o.UpdatedAt = &v
}

// GetDeletedAt returns the DeletedAt field value if set, zero value otherwise.
func (o *Resource) GetDeletedAt() time.Time {
	if o == nil || IsNil(o.DeletedAt) {
		var ret time.Time
		return ret
	}
	return *o.DeletedAt
}

// GetDeletedAtOk returns a tuple with the DeletedAt field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *Resource) GetDeletedAtOk() (*time.Time, bool) {
	if o == nil || IsNil(o.DeletedAt) {
		return nil, false
	}
	return o.DeletedAt, true
}

// HasDeletedAt returns a boolean if a field has been set.
func (o *Resource) HasDeletedAt() bool {
	if o != nil && !IsNil(o.DeletedAt) {
		return true
	}

	return false
}

// SetDeletedAt gets a reference to the given time.Time and assigns it to the DeletedAt field.
func (o *Resource) SetDeletedAt(v time.Time) {
	o.DeletedAt = &v
}

// GetManifest returns the Manifest field value if set, zero value otherwise.
func (o *Resource) GetManifest() map[string]interface{} {
	if o == nil || IsNil(o.Manifest) {
		var ret map[string]interface{}
		return ret
	}
	return o.Manifest
}

// GetManifestOk returns a tuple with the Manifest field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *Resource) GetManifestOk() (map[string]interface{}, bool) {
	if o == nil || IsNil(o.Manifest) {
		return map[string]interface{}{}, false
	}
	return o.Manifest, true
}

// HasManifest returns a boolean if a field has been set.
func (o *Resource) HasManifest() bool {
	if o != nil && !IsNil(o.Manifest) {
		return true
	}

	return false
}

// SetManifest gets a reference to the given map[string]interface{} and assigns it to the Manifest field.
func (o *Resource) SetManifest(v map[string]interface{}) {
	o.Manifest = v
}

// GetDeleteOption returns the DeleteOption field value if set, zero value otherwise.
func (o *Resource) GetDeleteOption() map[string]interface{} {
	if o == nil || IsNil(o.DeleteOption) {
		var ret map[string]interface{}
		return ret
	}
	return o.DeleteOption
}

// GetDeleteOptionOk returns a tuple with the DeleteOption field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *Resource) GetDeleteOptionOk() (map[string]interface{}, bool) {
	if o == nil || IsNil(o.DeleteOption) {
		return map[string]interface{}{}, false
	}
	return o.DeleteOption, true
}

// HasDeleteOption returns a boolean if a field has been set.
func (o *Resource) HasDeleteOption() bool {
	if o != nil && !IsNil(o.DeleteOption) {
		return true
	}

	return false
}

// SetDeleteOption gets a reference to the given map[string]interface{} and assigns it to the DeleteOption field.
func (o *Resource) SetDeleteOption(v map[string]interface{}) {
	o.DeleteOption = v
}

// GetUpdateStrategy returns the UpdateStrategy field value if set, zero value otherwise.
func (o *Resource) GetUpdateStrategy() map[string]interface{} {
	if o == nil || IsNil(o.UpdateStrategy) {
		var ret map[string]interface{}
		return ret
	}
	return o.UpdateStrategy
}

// GetUpdateStrategyOk returns a tuple with the UpdateStrategy field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *Resource) GetUpdateStrategyOk() (map[string]interface{}, bool) {
	if o == nil || IsNil(o.UpdateStrategy) {
		return map[string]interface{}{}, false
	}
	return o.UpdateStrategy, true
}

// HasUpdateStrategy returns a boolean if a field has been set.
func (o *Resource) HasUpdateStrategy() bool {
	if o != nil && !IsNil(o.UpdateStrategy) {
		return true
	}

	return false
}

// SetUpdateStrategy gets a reference to the given map[string]interface{} and assigns it to the UpdateStrategy field.
func (o *Resource) SetUpdateStrategy(v map[string]interface{}) {
	o.UpdateStrategy = v
}

// GetStatus returns the Status field value if set, zero value otherwise.
func (o *Resource) GetStatus() map[string]interface{} {
	if o == nil || IsNil(o.Status) {
		var ret map[string]interface{}
		return ret
	}
	return o.Status
}

// GetStatusOk returns a tuple with the Status field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *Resource) GetStatusOk() (map[string]interface{}, bool) {
	if o == nil || IsNil(o.Status) {
		return map[string]interface{}{}, false
	}
	return o.Status, true
}

// HasStatus returns a boolean if a field has been set.
func (o *Resource) HasStatus() bool {
	if o != nil && !IsNil(o.Status) {
		return true
	}

	return false
}

// SetStatus gets a reference to the given map[string]interface{} and assigns it to the Status field.
func (o *Resource) SetStatus(v map[string]interface{}) {
	o.Status = v
}

func (o Resource) MarshalJSON() ([]byte, error) {
	toSerialize, err := o.ToMap()
	if err != nil {
		return []byte{}, err
	}
	return json.Marshal(toSerialize)
}

func (o Resource) ToMap() (map[string]interface{}, error) {
	toSerialize := map[string]interface{}{}
	if !IsNil(o.Id) {
		toSerialize["id"] = o.Id
	}
	if !IsNil(o.Kind) {
		toSerialize["kind"] = o.Kind
	}
	if !IsNil(o.Href) {
		toSerialize["href"] = o.Href
	}
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
	if !IsNil(o.UpdateStrategy) {
		toSerialize["update_strategy"] = o.UpdateStrategy
	}
	if !IsNil(o.Status) {
		toSerialize["status"] = o.Status
	}
	return toSerialize, nil
}

type NullableResource struct {
	value *Resource
	isSet bool
}

func (v NullableResource) Get() *Resource {
	return v.value
}

func (v *NullableResource) Set(val *Resource) {
	v.value = val
	v.isSet = true
}

func (v NullableResource) IsSet() bool {
	return v.isSet
}

func (v *NullableResource) Unset() {
	v.value = nil
	v.isSet = false
}

func NewNullableResource(val *Resource) *NullableResource {
	return &NullableResource{value: val, isSet: true}
}

func (v NullableResource) MarshalJSON() ([]byte, error) {
	return json.Marshal(v.value)
}

func (v *NullableResource) UnmarshalJSON(src []byte) error {
	v.isSet = true
	return json.Unmarshal(src, &v.value)
}
