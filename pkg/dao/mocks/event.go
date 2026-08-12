package mocks

import (
	"context"
	"time"

	"gorm.io/gorm"

	"github.com/openshift-online/maestro/pkg/api"
	"github.com/openshift-online/maestro/pkg/dao"
)

var _ dao.EventDao = &eventDaoMock{}

type eventDaoMock struct {
	events api.EventList
}

func NewEventDao() *eventDaoMock {
	return &eventDaoMock{}
}

func (d *eventDaoMock) Get(ctx context.Context, id string) (*api.Event, error) {
	for _, event := range d.events {
		if event.ID == id {
			return event, nil
		}
	}
	return nil, gorm.ErrRecordNotFound
}

func (d *eventDaoMock) Create(ctx context.Context, event *api.Event) (*api.Event, error) {
	// mirror gorm's autoCreateTime so age-based logic (e.g. the delete-event republish
	// throttle) sees a realistic creation time instead of the zero value
	if event.CreatedAt.IsZero() {
		event.CreatedAt = time.Now()
	}
	d.events = append(d.events, event)
	return event, nil
}

func (d *eventDaoMock) Replace(ctx context.Context, event *api.Event) (*api.Event, error) {
	for i, e := range d.events {
		if e.ID == event.ID {
			d.events[i] = event
			return event, nil
		}
	}
	return nil, gorm.ErrRecordNotFound
}

func (d *eventDaoMock) Delete(ctx context.Context, id string) error {
	newEvents := api.EventList{}
	for _, e := range d.events {
		if e.ID == id {
			// deleting this one
			// do not include in the new list
		} else {
			newEvents = append(newEvents, e)
		}
	}
	d.events = newEvents
	return nil
}

func (d *eventDaoMock) FindByIDs(ctx context.Context, ids []string) (api.EventList, error) {
	filteredEvents := api.EventList{}
	for _, id := range ids {
		for _, e := range d.events {
			if e.ID == id {
				filteredEvents = append(filteredEvents, e)
			}
		}
	}
	return filteredEvents, nil
}

func (d *eventDaoMock) All(ctx context.Context) (api.EventList, error) {
	return d.events, nil
}

func (d *eventDaoMock) DeleteAllReconciledEvents(ctx context.Context) error {
	newEvents := api.EventList{}
	for _, e := range d.events {
		if e.ReconciledDate != nil {
			// deleting this one
			// do not include in the new list
		} else {
			newEvents = append(newEvents, e)
		}
	}
	d.events = newEvents
	return nil
}

func (d *eventDaoMock) FindAllUnreconciledEvents(ctx context.Context) (api.EventList, error) {
	filteredEvents := api.EventList{}
	for _, e := range d.events {
		if e.ReconciledDate != nil {
			continue
		}
		filteredEvents = append(filteredEvents, e)
	}

	return filteredEvents, nil
}

func (d *eventDaoMock) FindAgeOfOldestUnreconciledEvent(ctx context.Context) (*float64, error) {
	var oldest float64
	found := false
	now := time.Now()
	for _, e := range d.events {
		if e.ReconciledDate != nil {
			continue
		}
		found = true
		oldest = max(oldest, now.Sub(e.CreatedAt).Seconds())
	}
	if !found {
		return nil, nil
	}
	return &oldest, nil
}

func (d *eventDaoMock) ReconcileStaleDeleteEvents(ctx context.Context, cutoff time.Time) (int64, error) {
	// TODO: the mock has no resource state, so it cannot evaluate the resource soft-deleted
	// cutoff and ignores that half of the predicate. The event-age half is applied. Tests
	// that need to assert resource-cutoff filtering must use the integration test
	// (TestReconcileStaleDeleteEvents).
	now := time.Now()
	var count int64
	for _, e := range d.events {
		if e.ReconciledDate == nil && e.Source == "Resources" && e.EventType == api.DeleteEventType &&
			e.CreatedAt.Before(cutoff) {
			e.ReconciledDate = &now
			count++
		}
	}
	return count, nil
}

func (d *eventDaoMock) FindLatestDeleteEvent(ctx context.Context, sourceID string) (*api.Event, error) {
	var latest *api.Event
	for _, e := range d.events {
		if e.Source != "Resources" || e.SourceID != sourceID || e.EventType != api.DeleteEventType {
			continue
		}
		if latest == nil || e.CreatedAt.After(latest.CreatedAt) {
			latest = e
		}
	}
	return latest, nil
}
