package api

import "gorm.io/gorm"

type Resource struct {
	Meta
	Version    int32
	ConsumerID string
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
	d.ID = NewID()
	return nil
}

type ResourcePatchRequest struct{}
