package controllers

import (
	"context"
	"testing"

	. "github.com/onsi/gomega"
	"github.com/openshift-online/maestro/pkg/api"
	"github.com/openshift-online/maestro/pkg/dao/mocks"
	dbmocks "github.com/openshift-online/maestro/pkg/db/mocks"
	"github.com/openshift-online/maestro/pkg/services"
)

func TestLockingEventHandler(t *testing.T) {
	RegisterTestingT(t)

	source := "my-event-source"
	ctx := context.Background()
	eventsDao := mocks.NewEventDao()
	events := services.NewEventService(eventsDao)
	eventHandler := NewLockBasedEventHandler(dbmocks.NewMockAdvisoryLockFactory(), events)

	_, _ = eventsDao.Create(ctx, &api.Event{
		Meta:      api.Meta{ID: "1"},
		Source:    source,
		SourceID:  "any id",
		EventType: api.CreateEventType,
	})

	_, _ = eventsDao.Create(ctx, &api.Event{
		Meta:      api.Meta{ID: "2"},
		Source:    source,
		SourceID:  "any id",
		EventType: api.CreateEventType,
	})

	shouldProcess, err := eventHandler.ShouldHandleEvent(ctx, "1")
	Expect(err).To(BeNil())
	Expect(shouldProcess).To(BeTrue())

	lockingEventHandler, ok := eventHandler.(*LockBasedEventHandler)
	Expect(ok).To(BeTrue())
	Expect(lockingEventHandler.locks).To(HaveLen(1))

	eventHandler.DeferredAction(ctx, "1")
	Expect(lockingEventHandler.locks).To(HaveLen(0))

	event, err := eventsDao.Get(ctx, "1")
	Expect(err).To(BeNil())
	Expect(event.ReconciledDate).To(BeNil())

	err = eventHandler.PostProcess(ctx, event)
	Expect(err).To(BeNil())

	event, err = eventsDao.Get(ctx, "1")
	Expect(err).To(BeNil())
	Expect(event.ReconciledDate).NotTo(BeNil())

	shouldProcess, err = eventHandler.ShouldHandleEvent(ctx, "2")
	Expect(err).To(BeNil())
	Expect(shouldProcess).To(BeTrue())
	Expect(lockingEventHandler.locks).To(HaveLen(1))

	eventHandler.DeferredAction(ctx, "2")
	Expect(lockingEventHandler.locks).To(HaveLen(0))

	event, err = eventsDao.Get(ctx, "2")
	Expect(err).To(BeNil())
	Expect(event.ReconciledDate).To(BeNil())

	shouldProcess, err = eventHandler.ShouldHandleEvent(ctx, "3")
	Expect(err).To(BeNil())
	Expect(shouldProcess).To(BeTrue())
	Expect(lockingEventHandler.locks).To(HaveLen(1))
}

func TestPredicatedEventHandler(t *testing.T) {
	RegisterTestingT(t)

	currentInstanceID := "test-instance"
	anotherInstanceID := "another-instance"
	source := "my-event-source"
	ctx := context.Background()
	eventsDao := mocks.NewEventDao()
	events := services.NewEventService(eventsDao)
	eventInstancesDao := mocks.NewEventInstanceDaoMock()
	instancesDao := mocks.NewInstanceDao()
	eventServer := &exampleEventServer{eventDao: eventsDao}
	eventHandler := NewPredicatedEventHandler(eventServer.PredicateEvent, events, eventInstancesDao, instancesDao)

	// current instance is ready
	_, _ = instancesDao.Create(ctx, &api.ServerInstance{
		Meta:  api.Meta{ID: currentInstanceID},
		Ready: true,
	})

	// second instance is not ready
	_, _ = instancesDao.Create(ctx, &api.ServerInstance{
		Meta:  api.Meta{ID: anotherInstanceID},
		Ready: false,
	})

	_, _ = eventsDao.Create(ctx, &api.Event{
		Meta:      api.Meta{ID: "1"},
		Source:    source,
		SourceID:  "any id",
		EventType: api.CreateEventType,
	})

	_, _ = eventsDao.Create(ctx, &api.Event{
		Meta:      api.Meta{ID: "2"},
		Source:    source,
		SourceID:  "any id",
		EventType: api.CreateEventType,
	})

	// handle event 1
	shouldProcess, err := eventHandler.ShouldHandleEvent(ctx, "1")
	Expect(err).To(BeNil())
	Expect(shouldProcess).To(BeTrue())

	_, err = eventInstancesDao.Create(ctx, &api.EventInstance{
		SpecEventID: "1",
		InstanceID:  currentInstanceID,
	})
	Expect(err).To(BeNil())

	eventHandler.DeferredAction(ctx, "1")

	// simulate the second instance handled the event, although it has not been marked as ready
	_, err = eventInstancesDao.Create(ctx, &api.EventInstance{
		SpecEventID: "1",
		InstanceID:  anotherInstanceID,
	})
	Expect(err).To(BeNil())

	event, err := eventsDao.Get(ctx, "1")
	Expect(err).To(BeNil())
	Expect(event.ReconciledDate).To(BeNil())

	// should post process the event the second instance is not ready
	err = eventHandler.PostProcess(ctx, event)
	Expect(err).To(BeNil())
	Expect(event.ReconciledDate).NotTo(BeNil())

	// mark the second instance as ready
	err = instancesDao.MarkReadyByIDs(ctx, []string{anotherInstanceID})
	Expect(err).To(BeNil())

	// handle event 2
	shouldProcess, err = eventHandler.ShouldHandleEvent(ctx, "2")
	Expect(err).To(BeNil())
	Expect(shouldProcess).To(BeTrue())

	// simulate the current instance handled the event, the second instance is shutting down
	// before it handled the event
	_, err = eventInstancesDao.Create(ctx, &api.EventInstance{
		SpecEventID: "2",
		InstanceID:  currentInstanceID,
	})
	Expect(err).To(BeNil())

	eventHandler.DeferredAction(ctx, "2")

	event, err = eventsDao.Get(ctx, "2")
	Expect(err).To(BeNil())
	Expect(event.ReconciledDate).To(BeNil())

	err = eventHandler.PostProcess(ctx, event)
	Expect(err).NotTo(BeNil())
	Expect(event.ReconciledDate).To(BeNil())

	// mark the second instance as unready
	err = instancesDao.MarkUnreadyByIDs(ctx, []string{anotherInstanceID})
	Expect(err).To(BeNil())

	// simulate requeue the event
	err = eventHandler.PostProcess(ctx, event)
	Expect(err).To(BeNil())
	Expect(event.ReconciledDate).NotTo(BeNil())

	shouldProcess, err = eventHandler.ShouldHandleEvent(ctx, "3")
	Expect(err).NotTo(BeNil())
	Expect(shouldProcess).To(BeFalse())
}
