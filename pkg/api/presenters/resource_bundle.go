package presenters

import (
	"fmt"

	"github.com/openshift-online/maestro/pkg/api"
	"github.com/openshift-online/maestro/pkg/api/openapi"
)

// PresentResourceBundle converts a resource from the API to the openapi representation.
func PresentResourceBundle(resource *api.Resource) (*openapi.ResourceBundle, error) {
	metadata, manifests, manifestConfigs, deleteOption, err := api.DecodeManifestBundle(resource.Payload)
	if err != nil {
		return nil, err
	}
	status, err := api.DecodeBundleStatus(resource.Status)
	if err != nil {
		return nil, err
	}

	res := &openapi.ResourceBundle{
		Id:              openapi.PtrString(resource.ID),
		Kind:            openapi.PtrString("ResourceBundle"),
		Href:            openapi.PtrString(fmt.Sprintf("%s/%s/%s", BasePath, "resource-bundles", resource.ID)),
		Name:            openapi.PtrString(resource.Name),
		ConsumerName:    openapi.PtrString(resource.ConsumerName),
		Version:         openapi.PtrInt32(resource.Version),
		CreatedAt:       openapi.PtrTime(resource.CreatedAt),
		UpdatedAt:       openapi.PtrTime(resource.UpdatedAt),
		Metadata:        metadata,
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
