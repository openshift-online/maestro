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

func (d *eventInstanceDaoMock) GetInstancesByEventID(ctx context.Context, eventID string) ([]string, error) {
	d.mux.RLock()
	defer d.mux.RUnlock()

	var instanceIDs []string
	for _, ei := range d.eventInstances {
		if ei.EventID == eventID {
			instanceIDs = append(instanceIDs, ei.InstanceID)
		}
	}

	return instanceIDs, nil
}

func (d *eventInstanceDaoMock) Create(ctx context.Context, eventInstance *api.EventInstance) (*api.EventInstance, error) {
	d.mux.Lock()
	defer d.mux.Unlock()

	d.eventInstances = append(d.eventInstances, eventInstance)

	return eventInstance, nil
}
