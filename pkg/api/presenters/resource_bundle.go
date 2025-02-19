package presenters

import (
	"github.com/openshift-online/maestro/pkg/api"
	"github.com/openshift-online/maestro/pkg/api/openapi"
)

// PresentResourceBundle converts a resource from the API to the openapi representation.
func PresentResourceBundle(resource *api.Resource) (*openapi.ResourceBundle, error) {
	manifestWrapper, err := api.DecodeManifestBundle(resource.Payload)
	if err != nil {
		return nil, err
	}
	status, err := api.DecodeBundleStatus(resource.Status)
	if err != nil {
		return nil, err
	}

	reference := PresentReference(resource.ID, resource)
	rb := &openapi.ResourceBundle{
		Id:           reference.Id,
		Kind:         reference.Kind,
		Href:         reference.Href,
		Name:         openapi.PtrString(resource.Name),
		ConsumerName: openapi.PtrString(resource.ConsumerName),
		Version:      openapi.PtrInt32(resource.Version),
		CreatedAt:    openapi.PtrTime(resource.CreatedAt),
		UpdatedAt:    openapi.PtrTime(resource.UpdatedAt),
		Status:       status,
	}

	if manifestWrapper != nil {
		rb.Metadata = manifestWrapper.Meta
		rb.Manifests = manifestWrapper.Manifests
		rb.ManifestConfigs = manifestWrapper.ManifestConfigs
		rb.DeleteOption = manifestWrapper.DeleteOption
	}

	// set the deletedAt field if the resource has been marked as deleted
	if !resource.DeletedAt.Time.IsZero() {
		rb.DeletedAt = openapi.PtrTime(resource.DeletedAt.Time)
	}

	return rb, nil
}
