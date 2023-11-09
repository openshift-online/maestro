package presenters

import (
	"github.com/openshift-online/maestro/pkg/api"
	"github.com/openshift-online/maestro/pkg/api/openapi"
	"github.com/openshift-online/maestro/pkg/util"
)

func ConvertDinosaur(dinosaur openapi.Dinosaur) *api.Dinosaur {
	return &api.Dinosaur{
		Meta: api.Meta{
			ID: util.NilToEmptyString(dinosaur.Id),
		},
		Species: util.NilToEmptyString(dinosaur.Species),
	}
}

func PresentDinosaur(dinosaur *api.Dinosaur) openapi.Dinosaur {
	reference := PresentReference(dinosaur.ID, dinosaur)
	return openapi.Dinosaur{
		Id:        reference.Id,
		Kind:      reference.Kind,
		Href:      reference.Href,
		Species:   openapi.PtrString(dinosaur.Species),
		CreatedAt: openapi.PtrTime(dinosaur.CreatedAt),
		UpdatedAt: openapi.PtrTime(dinosaur.UpdatedAt),
	}
}
