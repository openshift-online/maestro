package presenters

import (
	"github.com/openshift-online/maestro/pkg/api"
	"github.com/openshift-online/maestro/pkg/api/openapi"
	"github.com/openshift-online/maestro/pkg/util"
)

func ConvertResource(resource openapi.Resource) *api.Resource {
	return &api.Resource{
		Meta: api.Meta{
			ID: util.NilToEmptyString(resource.Id),
		},
		ConsumerName: util.NilToEmptyString(resource.ConsumerName),
		Version:      util.NilToEmptyInt32(resource.Version),
		Manifest:     resource.Manifest,
		Status:       resource.Status,
	}
}

func PresentResource(resource *api.Resource) openapi.Resource {
	reference := PresentReference(resource.ID, resource)
	return openapi.Resource{
		Id:           reference.Id,
		Kind:         reference.Kind,
		Href:         reference.Href,
		ConsumerName: openapi.PtrString(resource.ConsumerName),
		Version:      openapi.PtrInt32(resource.Version),
		CreatedAt:    openapi.PtrTime(resource.CreatedAt),
		UpdatedAt:    openapi.PtrTime(resource.UpdatedAt),
		Manifest:     resource.Manifest,
		Status:       resource.Status,
	}
}
