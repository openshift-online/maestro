package api

import (
	"time"

	"gorm.io/datatypes"
	"gorm.io/gorm"
)

type StatusEventType string

const (
	StatusUpdateEventType StatusEventType = "StatusUpdate"
	StatusDeleteEventType StatusEventType = "StatusDelete"
)

type StatusEvent struct {
	Meta
	ResourceID      string
	ResourceSource  string
	ResourceType    ResourceType
	Payload         datatypes.JSONMap
	Status          datatypes.JSONMap
	StatusEventType StatusEventType // Update|Delete
	ReconciledDate  *time.Time      `json:"gorm:null"`
}

type StatusEventList []*StatusEvent
type StatusEventIndex map[string]*StatusEvent

func (l StatusEventList) Index() StatusEventIndex {
	index := StatusEventIndex{}
	for _, o := range l {
		index[o.ID] = o
	}
	return index
}

func (e *StatusEvent) BeforeCreate(tx *gorm.DB) error {
	e.ID = NewID()
	return nil
}
