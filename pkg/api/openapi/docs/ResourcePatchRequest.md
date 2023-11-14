# ResourcePatchRequest

## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**Manifest** | Pointer to **map[string]interface{}** |  | [optional] 
**Species** | Pointer to **string** |  | [optional] 

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

### GetSpecies

`func (o *ResourcePatchRequest) GetSpecies() string`

GetSpecies returns the Species field if non-nil, zero value otherwise.

### GetSpeciesOk

`func (o *ResourcePatchRequest) GetSpeciesOk() (*string, bool)`

GetSpeciesOk returns a tuple with the Species field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetSpecies

`func (o *ResourcePatchRequest) SetSpecies(v string)`

SetSpecies sets Species field to given value.

### HasSpecies

`func (o *ResourcePatchRequest) HasSpecies() bool`

HasSpecies returns a boolean if a field has been set.


[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


