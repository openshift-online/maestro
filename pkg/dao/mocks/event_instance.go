package mocks

import (
	"context"
	"fmt"
	"sync"

	"github.com/openshift-online/maestro/pkg/api"
	"github.com/openshift-online/maestro/pkg/dao"
)

var _ dao.EventInstanceDao = &eventInstanceDaoMock{}

type eventInstanceDaoMock struct {
	mux            sync.RWMutex
	eventInstances api.EventInstanceList
}

func NewEventInstanceDaoMock() *eventInstanceDaoMock {
	return &eventInstanceDaoMock{}
}

func (d *eventInstanceDaoMock) Get(ctx context.Context, eventID, instanceID string) (*api.EventInstance, error) {
	d.mux.RLock()
	defer d.mux.RUnlock()

	for _, ei := range d.eventInstances {
		if ei.EventID == eventID && ei.InstanceID == instanceID {
			return ei, nil
		}
	}

	return nil, fmt.Errorf("event instance not found")
}

func (d *eventInstanceDaoMock) Create(ctx context.Context, eventInstance *api.EventInstance) (*api.EventInstance, error) {
	d.mux.Lock()
	defer d.mux.Unlock()

	d.eventInstances = append(d.eventInstances, eventInstance)

	return eventInstance, nil
}

func (d *eventInstanceDaoMock) FindStatusEvents(ctx context.Context, ids []string) (api.EventInstanceList, error) {
	d.mux.RLock()
	defer d.mux.RUnlock()

	var eventInstances api.EventInstanceList
	for _, id := range ids {
		for _, ei := range d.eventInstances {
			if ei.EventID == id {
				eventInstances = append(eventInstances, ei)
			}
		}
	}

	return eventInstances, nil
}

func (d *eventInstanceDaoMock) GetEventsAssociatedWithInstances(ctx context.Context, instanceIDs []string) ([]string, error) {
	d.mux.RLock()
	defer d.mux.RUnlock()

	var eventIDs []string
	for _, ei := range d.eventInstances {
		if contains(instanceIDs, ei.InstanceID) {
			if ei.EventID == "" {
				continue
			}
			eventIDs = append(eventIDs, ei.EventID)
		}
	}

	return eventIDs, nil
}
