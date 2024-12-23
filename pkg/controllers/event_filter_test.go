package controllers

import (
	"context"
	"testing"

	"github.com/google/uuid"
	. "github.com/onsi/gomega"
	"github.com/openshift-online/maestro/pkg/api"
	"github.com/openshift-online/maestro/pkg/dao/mocks"
	dbmocks "github.com/openshift-online/maestro/pkg/db/mocks"
)

func TestLockingEventFilter(t *testing.T) {
	RegisterTestingT(t)

	source := "my-event-source"
	ctx := context.Background()
	eventsDao := mocks.NewEventDao()
	eventFilter := NewLockBasedEventFilter(dbmocks.NewMockAdvisoryLockFactory())

	_, err := eventsDao.Create(ctx, &api.Event{
		Meta:      api.Meta{ID: "1"},
		Source:    source,
		SourceID:  "any id",
		EventType: api.CreateEventType,
	})
	Expect(err).To(BeNil())

	_, err = eventsDao.Create(ctx, &api.Event{
		Meta:      api.Meta{ID: "2"},
		Source:    source,
		SourceID:  "any id",
		EventType: api.CreateEventType,
	})
	Expect(err).To(BeNil())

	shouldProcess, err := eventFilter.Filter(ctx, "1")
	Expect(err).To(BeNil())
	Expect(shouldProcess).To(BeTrue())

	lockingEventFilter, ok := eventFilter.(*LockBasedEventFilter)
	Expect(ok).To(BeTrue())
	Expect(lockingEventFilter.locks).To(HaveLen(1))

	eventFilter.DeferredAction(ctx, "1")
	Expect(lockingEventFilter.locks).To(HaveLen(0))

	event, err := eventsDao.Get(ctx, "1")
	Expect(err).To(BeNil())
	Expect(event.ReconciledDate).To(BeNil())

	shouldProcess, err = eventFilter.Filter(ctx, "2")
	Expect(err).To(BeNil())
	Expect(shouldProcess).To(BeTrue())
	Expect(lockingEventFilter.locks).To(HaveLen(1))

	eventFilter.DeferredAction(ctx, "2")
	Expect(lockingEventFilter.locks).To(HaveLen(0))

	event, err = eventsDao.Get(ctx, "2")
	Expect(err).To(BeNil())
	Expect(event.ReconciledDate).To(BeNil())

	shouldProcess, err = eventFilter.Filter(ctx, "3")
	Expect(err).To(BeNil())
	Expect(shouldProcess).To(BeTrue())
	Expect(lockingEventFilter.locks).To(HaveLen(1))
}

func TestPredicatedEventFilter(t *testing.T) {
	RegisterTestingT(t)

	source := "my-event-source"
	ctx := context.Background()
	eventsDao := mocks.NewEventDao()
	resourcesDao := mocks.NewResourceDao()
	eventServer := &exampleEventServer{eventsDao: eventsDao, resourcesDao: resourcesDao, subscrbers: []string{"cluster1"}}
	eventFilter := NewPredicatedEventFilter(eventServer.PredicateEvent)

	resID := uuid.New().String()
	_, err := resourcesDao.Create(ctx, &api.Resource{
		Meta:         api.Meta{ID: resID},
		ConsumerName: "cluster1",
		Source:       source,
	})
	Expect(err).To(BeNil())

	_, err = eventsDao.Create(ctx, &api.Event{
		Meta:      api.Meta{ID: "1"},
		Source:    source,
		SourceID:  resID,
		EventType: api.CreateEventType,
	})
	Expect(err).To(BeNil())

	_, err = eventsDao.Create(ctx, &api.Event{
		Meta:      api.Meta{ID: "2"},
		Source:    source,
		SourceID:  "any id",
		EventType: api.CreateEventType,
	})
	Expect(err).To(BeNil())

	newResID := uuid.New().String()
	_, err = resourcesDao.Create(ctx, &api.Resource{
		Meta:         api.Meta{ID: newResID},
		ConsumerName: "cluster2",
		Source:       source,
	})
	Expect(err).To(BeNil())

	_, err = eventsDao.Create(ctx, &api.Event{
		Meta:      api.Meta{ID: "3"},
		Source:    source,
		SourceID:  newResID,
		EventType: api.DeleteEventType,
	})
	Expect(err).To(BeNil())

	// handle event 1
	shouldProcess, err := eventFilter.Filter(ctx, "1")
	Expect(err).To(BeNil())
	Expect(shouldProcess).To(BeTrue())

	// call deferred action
	eventFilter.DeferredAction(ctx, "1")

	event, err := eventsDao.Get(ctx, "1")
	Expect(err).To(BeNil())
	Expect(event.ReconciledDate).To(BeNil())

	// handle event 2
	shouldProcess, err = eventFilter.Filter(ctx, "2")
	Expect(err).To(BeNil())
	Expect(shouldProcess).NotTo(BeTrue())

	event, err = eventsDao.Get(ctx, "2")
	Expect(err).To(BeNil())
	Expect(event.ReconciledDate).NotTo(BeNil())

	// handle event 3
	shouldProcess, err = eventFilter.Filter(ctx, "3")
	Expect(err).To(BeNil())
	Expect(shouldProcess).NotTo(BeTrue())

	event, err = eventsDao.Get(ctx, "3")
	Expect(err).To(BeNil())
	Expect(event.ReconciledDate).To(BeNil())
}
