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
	source := "my-event-source"
	ctx := context.Background()
	eventsDao := mocks.NewEventDao()
	events := services.NewEventService(eventsDao)
	eventInstancesDao := mocks.NewEventInstanceDaoMock()
	instancesDao := mocks.NewInstanceDao()
	eventServer := &exampleEventServer{eventDao: eventsDao}
	eventHandler := NewPredicatedEventHandler(eventServer.PredicateEvent, events, eventInstancesDao, instancesDao)

	_, _ = instancesDao.Create(ctx, &api.ServerInstance{
		Meta:  api.Meta{ID: currentInstanceID},
		Ready: true,
	})

	_, _ = instancesDao.Create(ctx, &api.ServerInstance{
		Meta:  api.Meta{ID: "another-instance"},
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

	shouldProcess, err := eventHandler.ShouldHandleEvent(ctx, "1")
	Expect(err).To(BeNil())
	Expect(shouldProcess).To(BeTrue())

	_, err = eventInstancesDao.Create(ctx, &api.EventInstance{
		EventID:    "1",
		InstanceID: currentInstanceID,
	})
	Expect(err).To(BeNil())

	eventHandler.DeferredAction(ctx, "1")

	event, err := eventsDao.Get(ctx, "1")
	Expect(err).To(BeNil())
	Expect(event.ReconciledDate).To(BeNil())

	err = eventHandler.PostProcess(ctx, event)
	Expect(err).To(BeNil())
	Expect(event.ReconciledDate).NotTo(BeNil())

	event, err = eventsDao.Get(ctx, "1")
	Expect(err).To(BeNil())
	Expect(event.ReconciledDate).NotTo(BeNil())

	shouldProcess, err = eventHandler.ShouldHandleEvent(ctx, "2")
	Expect(err).To(BeNil())
	Expect(shouldProcess).To(BeTrue())

	eventHandler.DeferredAction(ctx, "2")

	event, err = eventsDao.Get(ctx, "2")
	Expect(err).To(BeNil())
	Expect(event.ReconciledDate).To(BeNil())

	shouldProcess, err = eventHandler.ShouldHandleEvent(ctx, "3")
	Expect(err).NotTo(BeNil())
	Expect(shouldProcess).To(BeFalse())
}
