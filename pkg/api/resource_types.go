package api

import (
	"strconv"

	"gorm.io/datatypes"
	"gorm.io/gorm"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ktypes "k8s.io/apimachinery/pkg/types"
)

type ResourceType string

type Resource struct {
	Meta
	Version      int32
	Source       string
	ConsumerName string
	Type         ResourceType
	Payload      datatypes.JSONMap
	Status       datatypes.JSONMap
	// Name must be unique and not null, it can be treated as the resource external ID.
	// The format of the name should be follow the RFC 1123 (same as the k8s namespace).
	// When creating a resource, if its name is not specified, the resource id will be used as its name.
	// Cannot be updated.
	Name string
}

type ResourceList []*Resource
type ResourceIndex map[string]*Resource

func (l ResourceList) Index() ResourceIndex {
	index := ResourceIndex{}
	for _, o := range l {
		index[o.ID] = o
	}
	return index
}

func (d *Resource) BeforeCreate(tx *gorm.DB) error {
	// generate a new ID if it doesn't exist
	if d.ID == "" {
		d.ID = NewID()
	}
	if d.Name == "" {
		d.Name = d.ID
	}
	// start the resource version from 1
	if d.Version == 0 {
		d.Version = 1
	}
	return nil
}

func (d *Resource) GetUID() ktypes.UID {
	return ktypes.UID(d.Meta.ID)
}

func (d *Resource) GetResourceVersion() string {
	return strconv.FormatInt(int64(d.Version), 10)
}

func (d *Resource) GetDeletionTimestamp() *metav1.Time {
	return &metav1.Time{Time: d.Meta.DeletedAt.Time}
}
