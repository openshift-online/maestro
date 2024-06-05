package presenters

import (
	"encoding/json"
	"fmt"

	"gorm.io/datatypes"

	"github.com/openshift-online/maestro/pkg/api"
	"github.com/openshift-online/maestro/pkg/api/openapi"
	"github.com/openshift-online/maestro/pkg/constants"
	"github.com/openshift-online/maestro/pkg/util"
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
		// Set the default source ID for RESTful API calls and do not allow modification
		Source:   constants.DefaultSourceID,
		Type:     api.ResourceTypeSingle,
		Manifest: manifest,
	}, nil
}

// ConvertResourceManifest converts a resource manifest from the openapi representation to the API.
func ConvertResourceManifest(manifest, deleteOption, updateStrategy map[string]interface{}) (datatypes.JSONMap, error) {
	return api.EncodeManifest(manifest, deleteOption, updateStrategy)
}

// PresentResource converts a resource from the API to the openapi representation.
func PresentResource(resource *api.Resource) (*openapi.Resource, error) {
	manifest, deleteOption, updateStrategy, err := api.DecodeManifest(resource.Manifest)
	if err != nil {
		return nil, err
	}
	status, err := api.DecodeStatus(resource.Status)
	if err != nil {
		return nil, err
	}
	reference := PresentReference(resource.ID, resource)
	res := &openapi.Resource{
		Id:             reference.Id,
		Kind:           reference.Kind,
		Href:           reference.Href,
		Name:           openapi.PtrString(resource.Name),
		ConsumerName:   openapi.PtrString(resource.ConsumerName),
		Version:        openapi.PtrInt32(resource.Version),
		CreatedAt:      openapi.PtrTime(resource.CreatedAt),
		UpdatedAt:      openapi.PtrTime(resource.UpdatedAt),
		Manifest:       manifest,
		DeleteOption:   deleteOption,
		UpdateStrategy: updateStrategy,
		Status:         status,
	}

	// set the deletedAt field if the resource has been marked as deleted
	if !resource.DeletedAt.Time.IsZero() {
		res.DeletedAt = openapi.PtrTime(resource.DeletedAt.Time)
	}

	return res, nil
}

// PresentResourceBundle converts a resource from the API to the openapi representation.
func PresentResourceBundle(resource *api.Resource) (*openapi.ResourceBundle, error) {
	manifestBundle, err := api.DecodeManifestBundle(resource.Manifest)
	if err != nil {
		return nil, err
	}
	status, err := api.DecodeBundleStatus(resource.Status)
	if err != nil {
		return nil, err
	}

	reference := openapi.ObjectReference{
		Id:   openapi.PtrString(resource.ID),
		Kind: openapi.PtrString("ResourceBundle"),
		Href: openapi.PtrString(fmt.Sprintf("%s/%s/%s", BasePath, "resource-bundles", resource.ID)),
	}

	manifests := make([]map[string]interface{}, 0, len(manifestBundle.Manifests))
	for _, manifest := range manifestBundle.Manifests {
		mbytes, err := json.Marshal(manifest)
		if err != nil {
			return nil, err
		}
		m := map[string]interface{}{}
		if err := json.Unmarshal(mbytes, &m); err != nil {
			return nil, err
		}
		manifests = append(manifests, m)
	}
	deleteOption := map[string]interface{}{}
	if manifestBundle.DeleteOption != nil {
		dbytes, err := json.Marshal(manifestBundle.DeleteOption)
		if err != nil {
			return nil, err
		}
		if err := json.Unmarshal(dbytes, &deleteOption); err != nil {
			return nil, err
		}
	}
	manifestConfigs := make([]map[string]interface{}, 0, len(manifestBundle.ManifestConfigs))
	for _, manifestConfig := range manifestBundle.ManifestConfigs {
		mbytes, err := json.Marshal(manifestConfig)
		if err != nil {
			return nil, err
		}
		m := map[string]interface{}{}
		if err := json.Unmarshal(mbytes, &m); err != nil {
			return nil, err
		}
		manifestConfigs = append(manifestConfigs, m)
	}
	res := &openapi.ResourceBundle{
		Id:              reference.Id,
		Kind:            reference.Kind,
		Href:            reference.Href,
		Name:            openapi.PtrString(resource.Name),
		ConsumerName:    openapi.PtrString(resource.ConsumerName),
		Version:         openapi.PtrInt32(resource.Version),
		CreatedAt:       openapi.PtrTime(resource.CreatedAt),
		UpdatedAt:       openapi.PtrTime(resource.UpdatedAt),
		Manifests:       manifests,
		DeleteOption:    deleteOption,
		ManifestConfigs: manifestConfigs,
		Status:          status,
	}

	// set the deletedAt field if the resource has been marked as deleted
	if !resource.DeletedAt.Time.IsZero() {
		res.DeletedAt = openapi.PtrTime(resource.DeletedAt.Time)
	}

	return res, nil
}
