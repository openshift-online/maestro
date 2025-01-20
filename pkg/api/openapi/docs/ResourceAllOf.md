# ResourceAllOf

## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**Name** | Pointer to **string** |  | [optional] 
**ConsumerName** | Pointer to **string** |  | [optional] 
**Version** | Pointer to **int32** |  | [optional] 
**CreatedAt** | Pointer to **time.Time** |  | [optional] 
**UpdatedAt** | Pointer to **time.Time** |  | [optional] 
**DeletedAt** | Pointer to **time.Time** |  | [optional] 
**Manifest** | Pointer to **map[string]interface{}** |  | [optional] 
**DeleteOption** | Pointer to **map[string]interface{}** |  | [optional] 
**ManifestConfig** | Pointer to **map[string]interface{}** |  | [optional] 
**Status** | Pointer to **map[string]interface{}** |  | [optional] 

## Methods

### NewResourceAllOf

`func NewResourceAllOf() *ResourceAllOf`

NewResourceAllOf instantiates a new ResourceAllOf object
This constructor will assign default values to properties that have it defined,
and makes sure properties required by API are set, but the set of arguments
will change when the set of required properties is changed

### NewResourceAllOfWithDefaults

`func NewResourceAllOfWithDefaults() *ResourceAllOf`

NewResourceAllOfWithDefaults instantiates a new ResourceAllOf object
This constructor will only assign default values to properties that have it defined,
but it doesn't guarantee that properties required by API are set

### GetName

`func (o *ResourceAllOf) GetName() string`

GetName returns the Name field if non-nil, zero value otherwise.

### GetNameOk

`func (o *ResourceAllOf) GetNameOk() (*string, bool)`

GetNameOk returns a tuple with the Name field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetName

`func (o *ResourceAllOf) SetName(v string)`

SetName sets Name field to given value.

### HasName

`func (o *ResourceAllOf) HasName() bool`

HasName returns a boolean if a field has been set.

### GetConsumerName

`func (o *ResourceAllOf) GetConsumerName() string`

GetConsumerName returns the ConsumerName field if non-nil, zero value otherwise.

### GetConsumerNameOk

`func (o *ResourceAllOf) GetConsumerNameOk() (*string, bool)`

GetConsumerNameOk returns a tuple with the ConsumerName field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetConsumerName

`func (o *ResourceAllOf) SetConsumerName(v string)`

SetConsumerName sets ConsumerName field to given value.

### HasConsumerName

`func (o *ResourceAllOf) HasConsumerName() bool`

HasConsumerName returns a boolean if a field has been set.

### GetVersion

`func (o *ResourceAllOf) GetVersion() int32`

GetVersion returns the Version field if non-nil, zero value otherwise.

### GetVersionOk

`func (o *ResourceAllOf) GetVersionOk() (*int32, bool)`

GetVersionOk returns a tuple with the Version field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetVersion

`func (o *ResourceAllOf) SetVersion(v int32)`

SetVersion sets Version field to given value.

### HasVersion

`func (o *ResourceAllOf) HasVersion() bool`

HasVersion returns a boolean if a field has been set.

### GetCreatedAt

`func (o *ResourceAllOf) GetCreatedAt() time.Time`

GetCreatedAt returns the CreatedAt field if non-nil, zero value otherwise.

### GetCreatedAtOk

`func (o *ResourceAllOf) GetCreatedAtOk() (*time.Time, bool)`

GetCreatedAtOk returns a tuple with the CreatedAt field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetCreatedAt

`func (o *ResourceAllOf) SetCreatedAt(v time.Time)`

SetCreatedAt sets CreatedAt field to given value.

### HasCreatedAt

`func (o *ResourceAllOf) HasCreatedAt() bool`

HasCreatedAt returns a boolean if a field has been set.

### GetUpdatedAt

`func (o *ResourceAllOf) GetUpdatedAt() time.Time`

GetUpdatedAt returns the UpdatedAt field if non-nil, zero value otherwise.

### GetUpdatedAtOk

`func (o *ResourceAllOf) GetUpdatedAtOk() (*time.Time, bool)`

GetUpdatedAtOk returns a tuple with the UpdatedAt field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetUpdatedAt

`func (o *ResourceAllOf) SetUpdatedAt(v time.Time)`

SetUpdatedAt sets UpdatedAt field to given value.

### HasUpdatedAt

`func (o *ResourceAllOf) HasUpdatedAt() bool`

HasUpdatedAt returns a boolean if a field has been set.

### GetDeletedAt

`func (o *ResourceAllOf) GetDeletedAt() time.Time`

GetDeletedAt returns the DeletedAt field if non-nil, zero value otherwise.

### GetDeletedAtOk

`func (o *ResourceAllOf) GetDeletedAtOk() (*time.Time, bool)`

GetDeletedAtOk returns a tuple with the DeletedAt field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetDeletedAt

`func (o *ResourceAllOf) SetDeletedAt(v time.Time)`

SetDeletedAt sets DeletedAt field to given value.

### HasDeletedAt

`func (o *ResourceAllOf) HasDeletedAt() bool`

HasDeletedAt returns a boolean if a field has been set.

### GetManifest

`func (o *ResourceAllOf) GetManifest() map[string]interface{}`

GetManifest returns the Manifest field if non-nil, zero value otherwise.

### GetManifestOk

`func (o *ResourceAllOf) GetManifestOk() (*map[string]interface{}, bool)`

GetManifestOk returns a tuple with the Manifest field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetManifest

`func (o *ResourceAllOf) SetManifest(v map[string]interface{})`

SetManifest sets Manifest field to given value.

### HasManifest

`func (o *ResourceAllOf) HasManifest() bool`

HasManifest returns a boolean if a field has been set.

### GetDeleteOption

`func (o *ResourceAllOf) GetDeleteOption() map[string]interface{}`

GetDeleteOption returns the DeleteOption field if non-nil, zero value otherwise.

### GetDeleteOptionOk

`func (o *ResourceAllOf) GetDeleteOptionOk() (*map[string]interface{}, bool)`

GetDeleteOptionOk returns a tuple with the DeleteOption field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetDeleteOption

`func (o *ResourceAllOf) SetDeleteOption(v map[string]interface{})`

SetDeleteOption sets DeleteOption field to given value.

### HasDeleteOption

`func (o *ResourceAllOf) HasDeleteOption() bool`

HasDeleteOption returns a boolean if a field has been set.

### GetManifestConfig

`func (o *ResourceAllOf) GetManifestConfig() map[string]interface{}`

GetManifestConfig returns the ManifestConfig field if non-nil, zero value otherwise.

### GetManifestConfigOk

`func (o *ResourceAllOf) GetManifestConfigOk() (*map[string]interface{}, bool)`

GetManifestConfigOk returns a tuple with the ManifestConfig field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetManifestConfig

`func (o *ResourceAllOf) SetManifestConfig(v map[string]interface{})`

SetManifestConfig sets ManifestConfig field to given value.

### HasManifestConfig

`func (o *ResourceAllOf) HasManifestConfig() bool`

HasManifestConfig returns a boolean if a field has been set.

### GetStatus

`func (o *ResourceAllOf) GetStatus() map[string]interface{}`

GetStatus returns the Status field if non-nil, zero value otherwise.

### GetStatusOk

`func (o *ResourceAllOf) GetStatusOk() (*map[string]interface{}, bool)`

GetStatusOk returns a tuple with the Status field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetStatus

`func (o *ResourceAllOf) SetStatus(v map[string]interface{})`

SetStatus sets Status field to given value.

### HasStatus

`func (o *ResourceAllOf) HasStatus() bool`

HasStatus returns a boolean if a field has been set.


[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


