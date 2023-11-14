package api

import "gorm.io/gorm"

type Consumer struct {
	Meta
	Name string
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
	return nil
}

type ConsumerPatchRequest struct {
}
