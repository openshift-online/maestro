package controllers

import (
	"context"
	"testing"

	. "github.com/onsi/gomega"
	"github.com/openshift-online/maestro/pkg/api"
	"github.com/openshift-online/maestro/pkg/dao"
	"github.com/openshift-online/maestro/pkg/dao/mocks"
	dbmocks "github.com/openshift-online/maestro/pkg/db/mocks"
	"github.com/openshift-online/maestro/pkg/services"
)

func newExampleControllerConfig(ctrl *exampleController) *ControllerConfig {
	return &ControllerConfig{
		Source: "my-event-source",
		Handlers: map[api.EventType][]ControllerHandlerFunc{
			api.CreateEventType: {ctrl.OnAdd},
			api.UpdateEventType: {ctrl.OnUpdate},
			api.DeleteEventType: {ctrl.OnDelete},
		},
	}
}

type exampleController struct {
	instanceID        string
	eventInstancesDao dao.EventInstanceDao
	addCounter        int
	updateCounter     int
	deleteCounter     int
}

func (d *exampleController) OnAdd(ctx context.Context, eventID, resourceID string) error {
	d.addCounter++
	_, err := d.eventInstancesDao.Create(ctx, &api.EventInstance{
		SpecEventID: eventID,
		InstanceID:  d.instanceID,
	})
	return err
}

func (d *exampleController) OnUpdate(ctx context.Context, eventID, resourceID string) error {
	d.updateCounter++
	_, err := d.eventInstancesDao.Create(ctx, &api.EventInstance{
		SpecEventID: eventID,
		InstanceID:  d.instanceID,
	})
	return err
}

func (d *exampleController) OnDelete(ctx context.Context, eventID, resourceID string) error {
	d.deleteCounter++
	_, err := d.eventInstancesDao.Create(ctx, &api.EventInstance{
		SpecEventID: eventID,
		InstanceID:  d.instanceID,
	})
	return err
}

func TestControllerFrameworkWithLockBasedEventHandler(t *testing.T) {
	RegisterTestingT(t)

	ctx := context.Background()
	eventsDao := mocks.NewEventDao()
	events := services.NewEventService(eventsDao)
	eventInstancesDao := mocks.NewEventInstanceDaoMock()
	mgr := NewKindControllerManager(NewLockBasedEventHandler(dbmocks.NewMockAdvisoryLockFactory(), events), events)

	ctrl := &exampleController{
		instanceID:        "instance-1",
		eventInstancesDao: eventInstancesDao,
	}
	config := newExampleControllerConfig(ctrl)
	mgr.Add(config)

	_, _ = eventsDao.Create(ctx, &api.Event{
		Meta:      api.Meta{ID: "1"},
		Source:    config.Source,
		SourceID:  "any id",
		EventType: api.CreateEventType,
	})

	_, _ = eventsDao.Create(ctx, &api.Event{
		Meta:      api.Meta{ID: "2"},
		Source:    config.Source,
		SourceID:  "any id",
		EventType: api.UpdateEventType,
	})

	_, _ = eventsDao.Create(ctx, &api.Event{
		Meta:      api.Meta{ID: "3"},
		Source:    config.Source,
		SourceID:  "any id",
		EventType: api.DeleteEventType,
	})

	mgr.handleEvent("1")
	mgr.handleEvent("2")
	mgr.handleEvent("3")

	Expect(ctrl.addCounter).To(Equal(1))
	Expect(ctrl.updateCounter).To(Equal(1))
	Expect(ctrl.deleteCounter).To(Equal(1))

	eve, _ := eventsDao.Get(ctx, "1")
	Expect(eve.ReconciledDate).ToNot(BeNil(), "event reconcile date should be set")

	eve, _ = eventsDao.Get(ctx, "2")
	Expect(eve.ReconciledDate).ToNot(BeNil(), "event reconcile date should be set")

	eve, _ = eventsDao.Get(ctx, "3")
	Expect(eve.ReconciledDate).ToNot(BeNil(), "event reconcile date should be set")
}

type exampleEventServer struct {
	eventDao dao.EventDao
}

func (e *exampleEventServer) PredicateEvent(ctx context.Context, eventID string) (bool, error) {
	_, err := e.eventDao.Get(ctx, eventID)
	if err != nil {
		return false, err
	}
	return true, nil
}

func TestControllerFrameworkWithPredicatedEventHandler(t *testing.T) {
	RegisterTestingT(t)

	currentInstanceID := "test-instance"
	anotherInstanceID := "another-instance"
	ctx := context.Background()
	eventsDao := mocks.NewEventDao()
	events := services.NewEventService(eventsDao)
	eventServer := &exampleEventServer{eventDao: eventsDao}
	eventInstancesDao := mocks.NewEventInstanceDaoMock()
	instancesDao := mocks.NewInstanceDao()
	eventHandler := NewPredicatedEventHandler(eventServer.PredicateEvent, events, eventInstancesDao, instancesDao)
	mgr := NewKindControllerManager(eventHandler, events)

	ctrl := &exampleController{
		instanceID:        currentInstanceID,
		eventInstancesDao: eventInstancesDao,
	}
	config := newExampleControllerConfig(ctrl)
	mgr.Add(config)

	_, _ = instancesDao.Create(ctx, &api.ServerInstance{
		Meta:  api.Meta{ID: currentInstanceID},
		Ready: true,
	})

	_, _ = instancesDao.Create(ctx, &api.ServerInstance{
		Meta:  api.Meta{ID: anotherInstanceID},
		Ready: false,
	})

	_, _ = eventsDao.Create(ctx, &api.Event{
		Meta:      api.Meta{ID: "1"},
		Source:    config.Source,
		SourceID:  "any id",
		EventType: api.CreateEventType,
	})

	_, _ = eventsDao.Create(ctx, &api.Event{
		Meta:      api.Meta{ID: "2"},
		Source:    config.Source,
		SourceID:  "any id",
		EventType: api.UpdateEventType,
	})

	_, _ = eventsDao.Create(ctx, &api.Event{
		Meta:      api.Meta{ID: "3"},
		Source:    config.Source,
		SourceID:  "any id",
		EventType: api.DeleteEventType,
	})

	mgr.handleEvent("1")
	mgr.handleEvent("2")
	mgr.handleEvent("3")

	Expect(ctrl.addCounter).To(Equal(1))
	Expect(ctrl.updateCounter).To(Equal(1))
	Expect(ctrl.deleteCounter).To(Equal(1))

	eve, _ := eventsDao.Get(ctx, "1")
	Expect(eve.ReconciledDate).ToNot(BeNil(), "event reconcile date should be set")
}
