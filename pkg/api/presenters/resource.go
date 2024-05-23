package presenters

import (
	"github.com/openshift-online/maestro/pkg/api"
	"github.com/openshift-online/maestro/pkg/api/openapi"
	"github.com/openshift-online/maestro/pkg/util"
	"gorm.io/datatypes"
)

// ConvertResource converts a resource from the API to the openapi representation.
func ConvertResource(resource openapi.Resource) (*api.Resource, error) {
	manifest, err := ConvertResourceManifest(resource.Manifest, resource.DeleteOption, resource.UpdateStrategy)
	if err != nil {
		return nil, err
	}
	return &api.Resource{
		Name: util.NilToEmptyString(resource.Name),
		Meta: api.Meta{
			ID: util.NilToEmptyString(resource.Id),
		},
		ConsumerName: util.NilToEmptyString(resource.ConsumerName),
		Version:      util.NilToEmptyInt32(resource.Version),
		Type:         api.ResourceTypeSingle,
		Manifest:     manifest,
	}, nil
}

// ConvertResourceManifest converts a resource manifest from the openapi representation to the API.
func ConvertResourceManifest(manifest, deleteOption, updateStrategy map[string]interface{}) (datatypes.JSONMap, error) {
	return api.EncodeManifest(manifest, deleteOption, updateStrategy)
}

// PresentResource converts a resource from the API to the openapi representation.
func PresentResource(resource *api.Resource) (*openapi.Resource, error) {
	manifest, err := api.DecodeManifest(resource.Manifest)
	if err != nil {
		return nil, err
	}
	status, err := api.DecodeStatus(resource.Status)
	if err != nil {
		return nil, err
	}
	reference := PresentReference(resource.ID, resource)
	return &openapi.Resource{
		Id:           reference.Id,
		Kind:         reference.Kind,
		Href:         reference.Href,
		Name:         openapi.PtrString(resource.Name),
		ConsumerName: openapi.PtrString(resource.ConsumerName),
		Version:      openapi.PtrInt32(resource.Version),
		CreatedAt:    openapi.PtrTime(resource.CreatedAt),
		UpdatedAt:    openapi.PtrTime(resource.UpdatedAt),
		Manifest:     manifest,
		Status:       status,
	}, nil
}
