package api

import (
	"gorm.io/gorm"
	"time"
)

type EventType string

const (
	CreateEventType EventType = "Create"
	UpdateEventType EventType = "Update"
	DeleteEventType EventType = "Delete"
)

type Event struct {
	Meta
	Source         string     // MyTable
	SourceID       string     // primary key of MyTable
	EventType      EventType  // Add|Update|Delete
	ReconciledDate *time.Time `json:"gorm:null"`
}

type EventList []*Event
type EventIndex map[string]*Event

func (l EventList) Index() EventIndex {
	index := EventIndex{}
	for _, o := range l {
		index[o.ID] = o
	}
	return index
}

func (d *Event) BeforeCreate(tx *gorm.DB) error {
	d.ID = NewID()
	return nil
}
