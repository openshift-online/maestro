package api

import (
	"gorm.io/gorm"

	"github.com/openshift-online/maestro/pkg/db"
)

type Consumer struct {
	Meta

	// Name must be unique and not null, it can be treated as the consumer external ID.
	// The format of the name should be follow the RFC 1123 (same as the k8s namespace).
	// When creating a consumer, if its name is not specified, the consumer id will be used as its name.
	//
	// Cannot be updated.
	Name   string
	Labels *db.StringMap
}

type ConsumerList []*Consumer
type ConsumerIndex map[string]*Consumer

func (l ConsumerList) Index() ConsumerIndex {
	index := ConsumerIndex{}
	for _, o := range l {
		index[o.ID] = o
	}
	return index
}

func (d *Consumer) BeforeCreate(tx *gorm.DB) error {
	d.ID = NewID()

	if d.Name == "" {
		d.Name = d.ID
	}

	return nil
}

type ConsumerPatchRequest struct {
}
