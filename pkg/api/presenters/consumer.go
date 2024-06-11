package presenters

import (
	"github.com/openshift-online/maestro/pkg/api"
	"github.com/openshift-online/maestro/pkg/api/openapi"
	"github.com/openshift-online/maestro/pkg/db"
	"github.com/openshift-online/maestro/pkg/util"
)

func ConvertConsumer(consumer openapi.Consumer) *api.Consumer {
	return &api.Consumer{
		Meta: api.Meta{
			ID: util.NilToEmptyString(consumer.Id),
		},
		Name:   util.NilToEmptyString(consumer.Name),
		Labels: db.EmptyMapToNilStringMap(consumer.Labels),
	}
}

func PresentConsumer(consumer *api.Consumer) openapi.Consumer {
	reference := PresentReference(consumer.ID, consumer)
	converted := openapi.Consumer{
		Id:        reference.Id,
		Kind:      reference.Kind,
		Href:      reference.Href,
		Name:      openapi.PtrString(consumer.Name),
		Labels:    consumer.Labels.ToMap(),
		CreatedAt: openapi.PtrTime(consumer.CreatedAt),
		UpdatedAt: openapi.PtrTime(consumer.UpdatedAt),
	}
	if !consumer.DeletedAt.Time.IsZero() {
		converted.DeletedAt = openapi.PtrTime(consumer.DeletedAt.Time)
	}
	return converted
}
