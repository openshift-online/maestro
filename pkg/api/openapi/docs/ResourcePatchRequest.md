# ResourcePatchRequest

## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**Version** | Pointer to **int32** |  | [optional] 
**Manifest** | Pointer to **map[string]interface{}** |  | [optional] 
**DeleteOption** | Pointer to **map[string]interface{}** |  | [optional] 
**ManifestConfig** | Pointer to **map[string]interface{}** |  | [optional] 

## Methods

### NewResourcePatchRequest

`func NewResourcePatchRequest() *ResourcePatchRequest`

NewResourcePatchRequest instantiates a new ResourcePatchRequest object
This constructor will assign default values to properties that have it defined,
and makes sure properties required by API are set, but the set of arguments
will change when the set of required properties is changed

### NewResourcePatchRequestWithDefaults

`func NewResourcePatchRequestWithDefaults() *ResourcePatchRequest`

NewResourcePatchRequestWithDefaults instantiates a new ResourcePatchRequest object
This constructor will only assign default values to properties that have it defined,
but it doesn't guarantee that properties required by API are set

### GetVersion

`func (o *ResourcePatchRequest) GetVersion() int32`

GetVersion returns the Version field if non-nil, zero value otherwise.

### GetVersionOk

`func (o *ResourcePatchRequest) GetVersionOk() (*int32, bool)`

GetVersionOk returns a tuple with the Version field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetVersion

`func (o *ResourcePatchRequest) SetVersion(v int32)`

SetVersion sets Version field to given value.

### HasVersion

`func (o *ResourcePatchRequest) HasVersion() bool`

HasVersion returns a boolean if a field has been set.

### GetManifest

`func (o *ResourcePatchRequest) GetManifest() map[string]interface{}`

GetManifest returns the Manifest field if non-nil, zero value otherwise.

### GetManifestOk

`func (o *ResourcePatchRequest) GetManifestOk() (*map[string]interface{}, bool)`

GetManifestOk returns a tuple with the Manifest field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetManifest

`func (o *ResourcePatchRequest) SetManifest(v map[string]interface{})`

SetManifest sets Manifest field to given value.

### HasManifest

`func (o *ResourcePatchRequest) HasManifest() bool`

HasManifest returns a boolean if a field has been set.

### GetDeleteOption

`func (o *ResourcePatchRequest) GetDeleteOption() map[string]interface{}`

GetDeleteOption returns the DeleteOption field if non-nil, zero value otherwise.

### GetDeleteOptionOk

`func (o *ResourcePatchRequest) GetDeleteOptionOk() (*map[string]interface{}, bool)`

GetDeleteOptionOk returns a tuple with the DeleteOption field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetDeleteOption

`func (o *ResourcePatchRequest) SetDeleteOption(v map[string]interface{})`

SetDeleteOption sets DeleteOption field to given value.

### HasDeleteOption

`func (o *ResourcePatchRequest) HasDeleteOption() bool`

HasDeleteOption returns a boolean if a field has been set.

### GetManifestConfig

`func (o *ResourcePatchRequest) GetManifestConfig() map[string]interface{}`

GetManifestConfig returns the ManifestConfig field if non-nil, zero value otherwise.

### GetManifestConfigOk

`func (o *ResourcePatchRequest) GetManifestConfigOk() (*map[string]interface{}, bool)`

GetManifestConfigOk returns a tuple with the ManifestConfig field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetManifestConfig

`func (o *ResourcePatchRequest) SetManifestConfig(v map[string]interface{})`

SetManifestConfig sets ManifestConfig field to given value.

### HasManifestConfig

`func (o *ResourcePatchRequest) HasManifestConfig() bool`

HasManifestConfig returns a boolean if a field has been set.


[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


