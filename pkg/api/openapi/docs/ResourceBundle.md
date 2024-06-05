# ResourceBundle

## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**Id** | Pointer to **string** |  | [optional] 
**Kind** | Pointer to **string** |  | [optional] 
**Href** | Pointer to **string** |  | [optional] 
**Name** | Pointer to **string** |  | [optional] 
**ConsumerName** | Pointer to **string** |  | [optional] 
**Version** | Pointer to **int32** |  | [optional] 
**CreatedAt** | Pointer to **time.Time** |  | [optional] 
**UpdatedAt** | Pointer to **time.Time** |  | [optional] 
**DeletedAt** | Pointer to **time.Time** |  | [optional] 
**Manifests** | Pointer to **[]map[string]interface{}** |  | [optional] 
**DeleteOption** | Pointer to **map[string]interface{}** |  | [optional] 
**ManifestConfigs** | Pointer to **[]map[string]interface{}** |  | [optional] 
**Status** | Pointer to **map[string]interface{}** |  | [optional] 

## Methods

### NewResourceBundle

`func NewResourceBundle() *ResourceBundle`

NewResourceBundle instantiates a new ResourceBundle object
This constructor will assign default values to properties that have it defined,
and makes sure properties required by API are set, but the set of arguments
will change when the set of required properties is changed

### NewResourceBundleWithDefaults

`func NewResourceBundleWithDefaults() *ResourceBundle`

NewResourceBundleWithDefaults instantiates a new ResourceBundle object
This constructor will only assign default values to properties that have it defined,
but it doesn't guarantee that properties required by API are set

### GetId

`func (o *ResourceBundle) GetId() string`

GetId returns the Id field if non-nil, zero value otherwise.

### GetIdOk

`func (o *ResourceBundle) GetIdOk() (*string, bool)`

GetIdOk returns a tuple with the Id field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetId

`func (o *ResourceBundle) SetId(v string)`

SetId sets Id field to given value.

### HasId

`func (o *ResourceBundle) HasId() bool`

HasId returns a boolean if a field has been set.

### GetKind

`func (o *ResourceBundle) GetKind() string`

GetKind returns the Kind field if non-nil, zero value otherwise.

### GetKindOk

`func (o *ResourceBundle) GetKindOk() (*string, bool)`

GetKindOk returns a tuple with the Kind field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetKind

`func (o *ResourceBundle) SetKind(v string)`

SetKind sets Kind field to given value.

### HasKind

`func (o *ResourceBundle) HasKind() bool`

HasKind returns a boolean if a field has been set.

### GetHref

`func (o *ResourceBundle) GetHref() string`

GetHref returns the Href field if non-nil, zero value otherwise.

### GetHrefOk

`func (o *ResourceBundle) GetHrefOk() (*string, bool)`

GetHrefOk returns a tuple with the Href field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetHref

`func (o *ResourceBundle) SetHref(v string)`

SetHref sets Href field to given value.

### HasHref

`func (o *ResourceBundle) HasHref() bool`

HasHref returns a boolean if a field has been set.

### GetName

`func (o *ResourceBundle) GetName() string`

GetName returns the Name field if non-nil, zero value otherwise.

### GetNameOk

`func (o *ResourceBundle) GetNameOk() (*string, bool)`

GetNameOk returns a tuple with the Name field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetName

`func (o *ResourceBundle) SetName(v string)`

SetName sets Name field to given value.

### HasName

`func (o *ResourceBundle) HasName() bool`

HasName returns a boolean if a field has been set.

### GetConsumerName

`func (o *ResourceBundle) GetConsumerName() string`

GetConsumerName returns the ConsumerName field if non-nil, zero value otherwise.

### GetConsumerNameOk

`func (o *ResourceBundle) GetConsumerNameOk() (*string, bool)`

GetConsumerNameOk returns a tuple with the ConsumerName field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetConsumerName

`func (o *ResourceBundle) SetConsumerName(v string)`

SetConsumerName sets ConsumerName field to given value.

### HasConsumerName

`func (o *ResourceBundle) HasConsumerName() bool`

HasConsumerName returns a boolean if a field has been set.

### GetVersion

`func (o *ResourceBundle) GetVersion() int32`

GetVersion returns the Version field if non-nil, zero value otherwise.

### GetVersionOk

`func (o *ResourceBundle) GetVersionOk() (*int32, bool)`

GetVersionOk returns a tuple with the Version field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetVersion

`func (o *ResourceBundle) SetVersion(v int32)`

SetVersion sets Version field to given value.

### HasVersion

`func (o *ResourceBundle) HasVersion() bool`

HasVersion returns a boolean if a field has been set.

### GetCreatedAt

`func (o *ResourceBundle) GetCreatedAt() time.Time`

GetCreatedAt returns the CreatedAt field if non-nil, zero value otherwise.

### GetCreatedAtOk

`func (o *ResourceBundle) GetCreatedAtOk() (*time.Time, bool)`

GetCreatedAtOk returns a tuple with the CreatedAt field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetCreatedAt

`func (o *ResourceBundle) SetCreatedAt(v time.Time)`

SetCreatedAt sets CreatedAt field to given value.

### HasCreatedAt

`func (o *ResourceBundle) HasCreatedAt() bool`

HasCreatedAt returns a boolean if a field has been set.

### GetUpdatedAt

`func (o *ResourceBundle) GetUpdatedAt() time.Time`

GetUpdatedAt returns the UpdatedAt field if non-nil, zero value otherwise.

### GetUpdatedAtOk

`func (o *ResourceBundle) GetUpdatedAtOk() (*time.Time, bool)`

GetUpdatedAtOk returns a tuple with the UpdatedAt field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetUpdatedAt

`func (o *ResourceBundle) SetUpdatedAt(v time.Time)`

SetUpdatedAt sets UpdatedAt field to given value.

### HasUpdatedAt

`func (o *ResourceBundle) HasUpdatedAt() bool`

HasUpdatedAt returns a boolean if a field has been set.

### GetDeletedAt

`func (o *ResourceBundle) GetDeletedAt() time.Time`

GetDeletedAt returns the DeletedAt field if non-nil, zero value otherwise.

### GetDeletedAtOk

`func (o *ResourceBundle) GetDeletedAtOk() (*time.Time, bool)`

GetDeletedAtOk returns a tuple with the DeletedAt field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetDeletedAt

`func (o *ResourceBundle) SetDeletedAt(v time.Time)`

SetDeletedAt sets DeletedAt field to given value.

### HasDeletedAt

`func (o *ResourceBundle) HasDeletedAt() bool`

HasDeletedAt returns a boolean if a field has been set.

### GetManifests

`func (o *ResourceBundle) GetManifests() []map[string]interface{}`

GetManifests returns the Manifests field if non-nil, zero value otherwise.

### GetManifestsOk

`func (o *ResourceBundle) GetManifestsOk() (*[]map[string]interface{}, bool)`

GetManifestsOk returns a tuple with the Manifests field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetManifests

`func (o *ResourceBundle) SetManifests(v []map[string]interface{})`

SetManifests sets Manifests field to given value.

### HasManifests

`func (o *ResourceBundle) HasManifests() bool`

HasManifests returns a boolean if a field has been set.

### GetDeleteOption

`func (o *ResourceBundle) GetDeleteOption() map[string]interface{}`

GetDeleteOption returns the DeleteOption field if non-nil, zero value otherwise.

### GetDeleteOptionOk

`func (o *ResourceBundle) GetDeleteOptionOk() (*map[string]interface{}, bool)`

GetDeleteOptionOk returns a tuple with the DeleteOption field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetDeleteOption

`func (o *ResourceBundle) SetDeleteOption(v map[string]interface{})`

SetDeleteOption sets DeleteOption field to given value.

### HasDeleteOption

`func (o *ResourceBundle) HasDeleteOption() bool`

HasDeleteOption returns a boolean if a field has been set.

### GetManifestConfigs

`func (o *ResourceBundle) GetManifestConfigs() []map[string]interface{}`

GetManifestConfigs returns the ManifestConfigs field if non-nil, zero value otherwise.

### GetManifestConfigsOk

`func (o *ResourceBundle) GetManifestConfigsOk() (*[]map[string]interface{}, bool)`

GetManifestConfigsOk returns a tuple with the ManifestConfigs field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetManifestConfigs

`func (o *ResourceBundle) SetManifestConfigs(v []map[string]interface{})`

SetManifestConfigs sets ManifestConfigs field to given value.

### HasManifestConfigs

`func (o *ResourceBundle) HasManifestConfigs() bool`

HasManifestConfigs returns a boolean if a field has been set.

### GetStatus

`func (o *ResourceBundle) GetStatus() map[string]interface{}`

GetStatus returns the Status field if non-nil, zero value otherwise.

### GetStatusOk

`func (o *ResourceBundle) GetStatusOk() (*map[string]interface{}, bool)`

GetStatusOk returns a tuple with the Status field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetStatus

`func (o *ResourceBundle) SetStatus(v map[string]interface{})`

SetStatus sets Status field to given value.

### HasStatus

`func (o *ResourceBundle) HasStatus() bool`

HasStatus returns a boolean if a field has been set.


[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


