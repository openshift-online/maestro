# Resource

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
**Manifest** | Pointer to **map[string]interface{}** |  | [optional] 
**DeleteOption** | Pointer to **map[string]interface{}** |  | [optional] 
**ManifestConfig** | Pointer to **map[string]interface{}** |  | [optional] 
**Status** | Pointer to **map[string]interface{}** |  | [optional] 

## Methods

### NewResource

`func NewResource() *Resource`

NewResource instantiates a new Resource object
This constructor will assign default values to properties that have it defined,
and makes sure properties required by API are set, but the set of arguments
will change when the set of required properties is changed

### NewResourceWithDefaults

`func NewResourceWithDefaults() *Resource`

NewResourceWithDefaults instantiates a new Resource object
This constructor will only assign default values to properties that have it defined,
but it doesn't guarantee that properties required by API are set

### GetId

`func (o *Resource) GetId() string`

GetId returns the Id field if non-nil, zero value otherwise.

### GetIdOk

`func (o *Resource) GetIdOk() (*string, bool)`

GetIdOk returns a tuple with the Id field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetId

`func (o *Resource) SetId(v string)`

SetId sets Id field to given value.

### HasId

`func (o *Resource) HasId() bool`

HasId returns a boolean if a field has been set.

### GetKind

`func (o *Resource) GetKind() string`

GetKind returns the Kind field if non-nil, zero value otherwise.

### GetKindOk

`func (o *Resource) GetKindOk() (*string, bool)`

GetKindOk returns a tuple with the Kind field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetKind

`func (o *Resource) SetKind(v string)`

SetKind sets Kind field to given value.

### HasKind

`func (o *Resource) HasKind() bool`

HasKind returns a boolean if a field has been set.

### GetHref

`func (o *Resource) GetHref() string`

GetHref returns the Href field if non-nil, zero value otherwise.

### GetHrefOk

`func (o *Resource) GetHrefOk() (*string, bool)`

GetHrefOk returns a tuple with the Href field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetHref

`func (o *Resource) SetHref(v string)`

SetHref sets Href field to given value.

### HasHref

`func (o *Resource) HasHref() bool`

HasHref returns a boolean if a field has been set.

### GetName

`func (o *Resource) GetName() string`

GetName returns the Name field if non-nil, zero value otherwise.

### GetNameOk

`func (o *Resource) GetNameOk() (*string, bool)`

GetNameOk returns a tuple with the Name field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetName

`func (o *Resource) SetName(v string)`

SetName sets Name field to given value.

### HasName

`func (o *Resource) HasName() bool`

HasName returns a boolean if a field has been set.

### GetConsumerName

`func (o *Resource) GetConsumerName() string`

GetConsumerName returns the ConsumerName field if non-nil, zero value otherwise.

### GetConsumerNameOk

`func (o *Resource) GetConsumerNameOk() (*string, bool)`

GetConsumerNameOk returns a tuple with the ConsumerName field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetConsumerName

`func (o *Resource) SetConsumerName(v string)`

SetConsumerName sets ConsumerName field to given value.

### HasConsumerName

`func (o *Resource) HasConsumerName() bool`

HasConsumerName returns a boolean if a field has been set.

### GetVersion

`func (o *Resource) GetVersion() int32`

GetVersion returns the Version field if non-nil, zero value otherwise.

### GetVersionOk

`func (o *Resource) GetVersionOk() (*int32, bool)`

GetVersionOk returns a tuple with the Version field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetVersion

`func (o *Resource) SetVersion(v int32)`

SetVersion sets Version field to given value.

### HasVersion

`func (o *Resource) HasVersion() bool`

HasVersion returns a boolean if a field has been set.

### GetCreatedAt

`func (o *Resource) GetCreatedAt() time.Time`

GetCreatedAt returns the CreatedAt field if non-nil, zero value otherwise.

### GetCreatedAtOk

`func (o *Resource) GetCreatedAtOk() (*time.Time, bool)`

GetCreatedAtOk returns a tuple with the CreatedAt field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetCreatedAt

`func (o *Resource) SetCreatedAt(v time.Time)`

SetCreatedAt sets CreatedAt field to given value.

### HasCreatedAt

`func (o *Resource) HasCreatedAt() bool`

HasCreatedAt returns a boolean if a field has been set.

### GetUpdatedAt

`func (o *Resource) GetUpdatedAt() time.Time`

GetUpdatedAt returns the UpdatedAt field if non-nil, zero value otherwise.

### GetUpdatedAtOk

`func (o *Resource) GetUpdatedAtOk() (*time.Time, bool)`

GetUpdatedAtOk returns a tuple with the UpdatedAt field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetUpdatedAt

`func (o *Resource) SetUpdatedAt(v time.Time)`

SetUpdatedAt sets UpdatedAt field to given value.

### HasUpdatedAt

`func (o *Resource) HasUpdatedAt() bool`

HasUpdatedAt returns a boolean if a field has been set.

### GetDeletedAt

`func (o *Resource) GetDeletedAt() time.Time`

GetDeletedAt returns the DeletedAt field if non-nil, zero value otherwise.

### GetDeletedAtOk

`func (o *Resource) GetDeletedAtOk() (*time.Time, bool)`

GetDeletedAtOk returns a tuple with the DeletedAt field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetDeletedAt

`func (o *Resource) SetDeletedAt(v time.Time)`

SetDeletedAt sets DeletedAt field to given value.

### HasDeletedAt

`func (o *Resource) HasDeletedAt() bool`

HasDeletedAt returns a boolean if a field has been set.

### GetManifest

`func (o *Resource) GetManifest() map[string]interface{}`

GetManifest returns the Manifest field if non-nil, zero value otherwise.

### GetManifestOk

`func (o *Resource) GetManifestOk() (*map[string]interface{}, bool)`

GetManifestOk returns a tuple with the Manifest field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetManifest

`func (o *Resource) SetManifest(v map[string]interface{})`

SetManifest sets Manifest field to given value.

### HasManifest

`func (o *Resource) HasManifest() bool`

HasManifest returns a boolean if a field has been set.

### GetDeleteOption

`func (o *Resource) GetDeleteOption() map[string]interface{}`

GetDeleteOption returns the DeleteOption field if non-nil, zero value otherwise.

### GetDeleteOptionOk

`func (o *Resource) GetDeleteOptionOk() (*map[string]interface{}, bool)`

GetDeleteOptionOk returns a tuple with the DeleteOption field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetDeleteOption

`func (o *Resource) SetDeleteOption(v map[string]interface{})`

SetDeleteOption sets DeleteOption field to given value.

### HasDeleteOption

`func (o *Resource) HasDeleteOption() bool`

HasDeleteOption returns a boolean if a field has been set.

### GetManifestConfig

`func (o *Resource) GetManifestConfig() map[string]interface{}`

GetManifestConfig returns the ManifestConfig field if non-nil, zero value otherwise.

### GetManifestConfigOk

`func (o *Resource) GetManifestConfigOk() (*map[string]interface{}, bool)`

GetManifestConfigOk returns a tuple with the ManifestConfig field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetManifestConfig

`func (o *Resource) SetManifestConfig(v map[string]interface{})`

SetManifestConfig sets ManifestConfig field to given value.

### HasManifestConfig

`func (o *Resource) HasManifestConfig() bool`

HasManifestConfig returns a boolean if a field has been set.

### GetStatus

`func (o *Resource) GetStatus() map[string]interface{}`

GetStatus returns the Status field if non-nil, zero value otherwise.

### GetStatusOk

`func (o *Resource) GetStatusOk() (*map[string]interface{}, bool)`

GetStatusOk returns a tuple with the Status field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetStatus

`func (o *Resource) SetStatus(v map[string]interface{})`

SetStatus sets Status field to given value.

### HasStatus

`func (o *Resource) HasStatus() bool`

HasStatus returns a boolean if a field has been set.


[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


