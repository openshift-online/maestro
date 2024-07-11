# ResourceBundleAllOf

## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**Name** | Pointer to **string** |  | [optional] 
**ConsumerName** | Pointer to **string** |  | [optional] 
**Version** | Pointer to **int32** |  | [optional] 
**CreatedAt** | Pointer to **time.Time** |  | [optional] 
**UpdatedAt** | Pointer to **time.Time** |  | [optional] 
**DeletedAt** | Pointer to **time.Time** |  | [optional] 
**Metadata** | Pointer to **map[string]interface{}** |  | [optional] 
**Manifests** | Pointer to **[]map[string]interface{}** |  | [optional] 
**DeleteOption** | Pointer to **map[string]interface{}** |  | [optional] 
**ManifestConfigs** | Pointer to **[]map[string]interface{}** |  | [optional] 
**Status** | Pointer to **map[string]interface{}** |  | [optional] 

## Methods

### NewResourceBundleAllOf

`func NewResourceBundleAllOf() *ResourceBundleAllOf`

NewResourceBundleAllOf instantiates a new ResourceBundleAllOf object
This constructor will assign default values to properties that have it defined,
and makes sure properties required by API are set, but the set of arguments
will change when the set of required properties is changed

### NewResourceBundleAllOfWithDefaults

`func NewResourceBundleAllOfWithDefaults() *ResourceBundleAllOf`

NewResourceBundleAllOfWithDefaults instantiates a new ResourceBundleAllOf object
This constructor will only assign default values to properties that have it defined,
but it doesn't guarantee that properties required by API are set

### GetName

`func (o *ResourceBundleAllOf) GetName() string`

GetName returns the Name field if non-nil, zero value otherwise.

### GetNameOk

`func (o *ResourceBundleAllOf) GetNameOk() (*string, bool)`

GetNameOk returns a tuple with the Name field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetName

`func (o *ResourceBundleAllOf) SetName(v string)`

SetName sets Name field to given value.

### HasName

`func (o *ResourceBundleAllOf) HasName() bool`

HasName returns a boolean if a field has been set.

### GetConsumerName

`func (o *ResourceBundleAllOf) GetConsumerName() string`

GetConsumerName returns the ConsumerName field if non-nil, zero value otherwise.

### GetConsumerNameOk

`func (o *ResourceBundleAllOf) GetConsumerNameOk() (*string, bool)`

GetConsumerNameOk returns a tuple with the ConsumerName field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetConsumerName

`func (o *ResourceBundleAllOf) SetConsumerName(v string)`

SetConsumerName sets ConsumerName field to given value.

### HasConsumerName

`func (o *ResourceBundleAllOf) HasConsumerName() bool`

HasConsumerName returns a boolean if a field has been set.

### GetVersion

`func (o *ResourceBundleAllOf) GetVersion() int32`

GetVersion returns the Version field if non-nil, zero value otherwise.

### GetVersionOk

`func (o *ResourceBundleAllOf) GetVersionOk() (*int32, bool)`

GetVersionOk returns a tuple with the Version field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetVersion

`func (o *ResourceBundleAllOf) SetVersion(v int32)`

SetVersion sets Version field to given value.

### HasVersion

`func (o *ResourceBundleAllOf) HasVersion() bool`

HasVersion returns a boolean if a field has been set.

### GetCreatedAt

`func (o *ResourceBundleAllOf) GetCreatedAt() time.Time`

GetCreatedAt returns the CreatedAt field if non-nil, zero value otherwise.

### GetCreatedAtOk

`func (o *ResourceBundleAllOf) GetCreatedAtOk() (*time.Time, bool)`

GetCreatedAtOk returns a tuple with the CreatedAt field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetCreatedAt

`func (o *ResourceBundleAllOf) SetCreatedAt(v time.Time)`

SetCreatedAt sets CreatedAt field to given value.

### HasCreatedAt

`func (o *ResourceBundleAllOf) HasCreatedAt() bool`

HasCreatedAt returns a boolean if a field has been set.

### GetUpdatedAt

`func (o *ResourceBundleAllOf) GetUpdatedAt() time.Time`

GetUpdatedAt returns the UpdatedAt field if non-nil, zero value otherwise.

### GetUpdatedAtOk

`func (o *ResourceBundleAllOf) GetUpdatedAtOk() (*time.Time, bool)`

GetUpdatedAtOk returns a tuple with the UpdatedAt field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetUpdatedAt

`func (o *ResourceBundleAllOf) SetUpdatedAt(v time.Time)`

SetUpdatedAt sets UpdatedAt field to given value.

### HasUpdatedAt

`func (o *ResourceBundleAllOf) HasUpdatedAt() bool`

HasUpdatedAt returns a boolean if a field has been set.

### GetDeletedAt

`func (o *ResourceBundleAllOf) GetDeletedAt() time.Time`

GetDeletedAt returns the DeletedAt field if non-nil, zero value otherwise.

### GetDeletedAtOk

`func (o *ResourceBundleAllOf) GetDeletedAtOk() (*time.Time, bool)`

GetDeletedAtOk returns a tuple with the DeletedAt field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetDeletedAt

`func (o *ResourceBundleAllOf) SetDeletedAt(v time.Time)`

SetDeletedAt sets DeletedAt field to given value.

### HasDeletedAt

`func (o *ResourceBundleAllOf) HasDeletedAt() bool`

HasDeletedAt returns a boolean if a field has been set.

### GetMetadata

`func (o *ResourceBundleAllOf) GetMetadata() map[string]interface{}`

GetMetadata returns the Metadata field if non-nil, zero value otherwise.

### GetMetadataOk

`func (o *ResourceBundleAllOf) GetMetadataOk() (*map[string]interface{}, bool)`

GetMetadataOk returns a tuple with the Metadata field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetMetadata

`func (o *ResourceBundleAllOf) SetMetadata(v map[string]interface{})`

SetMetadata sets Metadata field to given value.

### HasMetadata

`func (o *ResourceBundleAllOf) HasMetadata() bool`

HasMetadata returns a boolean if a field has been set.

### GetManifests

`func (o *ResourceBundleAllOf) GetManifests() []map[string]interface{}`

GetManifests returns the Manifests field if non-nil, zero value otherwise.

### GetManifestsOk

`func (o *ResourceBundleAllOf) GetManifestsOk() (*[]map[string]interface{}, bool)`

GetManifestsOk returns a tuple with the Manifests field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetManifests

`func (o *ResourceBundleAllOf) SetManifests(v []map[string]interface{})`

SetManifests sets Manifests field to given value.

### HasManifests

`func (o *ResourceBundleAllOf) HasManifests() bool`

HasManifests returns a boolean if a field has been set.

### GetDeleteOption

`func (o *ResourceBundleAllOf) GetDeleteOption() map[string]interface{}`

GetDeleteOption returns the DeleteOption field if non-nil, zero value otherwise.

### GetDeleteOptionOk

`func (o *ResourceBundleAllOf) GetDeleteOptionOk() (*map[string]interface{}, bool)`

GetDeleteOptionOk returns a tuple with the DeleteOption field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetDeleteOption

`func (o *ResourceBundleAllOf) SetDeleteOption(v map[string]interface{})`

SetDeleteOption sets DeleteOption field to given value.

### HasDeleteOption

`func (o *ResourceBundleAllOf) HasDeleteOption() bool`

HasDeleteOption returns a boolean if a field has been set.

### GetManifestConfigs

`func (o *ResourceBundleAllOf) GetManifestConfigs() []map[string]interface{}`

GetManifestConfigs returns the ManifestConfigs field if non-nil, zero value otherwise.

### GetManifestConfigsOk

`func (o *ResourceBundleAllOf) GetManifestConfigsOk() (*[]map[string]interface{}, bool)`

GetManifestConfigsOk returns a tuple with the ManifestConfigs field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetManifestConfigs

`func (o *ResourceBundleAllOf) SetManifestConfigs(v []map[string]interface{})`

SetManifestConfigs sets ManifestConfigs field to given value.

### HasManifestConfigs

`func (o *ResourceBundleAllOf) HasManifestConfigs() bool`

HasManifestConfigs returns a boolean if a field has been set.

### GetStatus

`func (o *ResourceBundleAllOf) GetStatus() map[string]interface{}`

GetStatus returns the Status field if non-nil, zero value otherwise.

### GetStatusOk

`func (o *ResourceBundleAllOf) GetStatusOk() (*map[string]interface{}, bool)`

GetStatusOk returns a tuple with the Status field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetStatus

`func (o *ResourceBundleAllOf) SetStatus(v map[string]interface{})`

SetStatus sets Status field to given value.

### HasStatus

`func (o *ResourceBundleAllOf) HasStatus() bool`

HasStatus returns a boolean if a field has been set.


[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


