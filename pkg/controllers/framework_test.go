package controllers

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	. "github.com/onsi/gomega"
	"gorm.io/gorm"

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
	addCounter    int
	updateCounter int
	deleteCounter int
}

func (d *exampleController) OnAdd(ctx context.Context, id string) error {
	d.addCounter++
	return nil
}

func (d *exampleController) OnUpdate(ctx context.Context, id string) error {
	d.updateCounter++
	return nil
}

func (d *exampleController) OnDelete(ctx context.Context, id string) error {
	d.deleteCounter++
	return nil
}

func TestControllerFrameworkWithLockBasedEventFilter(t *testing.T) {
	RegisterTestingT(t)

	ctx := context.Background()
	eventsDao := mocks.NewEventDao()
	events := services.NewEventService(eventsDao)
	mgr := NewKindControllerManager(NewLockBasedEventFilter(dbmocks.NewMockAdvisoryLockFactory()), events)

	ctrl := &exampleController{}
	config := newExampleControllerConfig(ctrl)
	mgr.Add(config)

	_, err := eventsDao.Create(ctx, &api.Event{
		Meta:      api.Meta{ID: "1"},
		Source:    config.Source,
		SourceID:  "any id",
		EventType: api.CreateEventType,
	})
	Expect(err).To(BeNil())

	_, err = eventsDao.Create(ctx, &api.Event{
		Meta:      api.Meta{ID: "2"},
		Source:    config.Source,
		SourceID:  "any id",
		EventType: api.UpdateEventType,
	})
	Expect(err).To(BeNil())

	_, err = eventsDao.Create(ctx, &api.Event{
		Meta:      api.Meta{ID: "3"},
		Source:    config.Source,
		SourceID:  "any id",
		EventType: api.DeleteEventType,
	})
	Expect(err).To(BeNil())

	mgr.handleEvent(ctx, "1")
	mgr.handleEvent(ctx, "2")
	mgr.handleEvent(ctx, "3")

	Expect(ctrl.addCounter).To(Equal(1))
	Expect(ctrl.updateCounter).To(Equal(1))
	Expect(ctrl.deleteCounter).To(Equal(1))

	eve, err := eventsDao.Get(ctx, "1")
	Expect(err).To(BeNil())
	Expect(eve.ReconciledDate).ToNot(BeNil(), "event reconcile date should be set")

	eve, err = eventsDao.Get(ctx, "2")
	Expect(err).To(BeNil())
	Expect(eve.ReconciledDate).ToNot(BeNil(), "event reconcile date should be set")

	eve, err = eventsDao.Get(ctx, "3")
	Expect(err).To(BeNil())
	Expect(eve.ReconciledDate).ToNot(BeNil(), "event reconcile date should be set")
}

type exampleEventServer struct {
	eventsDao    dao.EventDao
	resourcesDao dao.ResourceDao
	subscrbers   []string
}

func (e *exampleEventServer) PredicateEvent(ctx context.Context, eventID string) (bool, error) {
	event, err := e.eventsDao.Get(ctx, eventID)
	if err != nil {
		return false, err
	}
	resource, err := e.resourcesDao.Get(ctx, event.SourceID)
	if err != nil {
		// 404 == gorm.ErrRecordNotFound  means the resource was deleted, so we can ignore the event
		if err == gorm.ErrRecordNotFound {
			now := time.Now()
			event.ReconciledDate = &now
			if _, svcErr := e.eventsDao.Replace(ctx, event); svcErr != nil {
				return false, fmt.Errorf("failed to update event %s: %s", event.ID, svcErr.Error())
			}
			return false, nil
		}
		return false, err
	}
	return contains(e.subscrbers, resource.ConsumerName), nil
}

func contains(s []string, e string) bool {
	for _, a := range s {
		if a == e {
			return true
		}
	}
	return false
}

func TestControllerFrameworkWithPredicatedEventFilter(t *testing.T) {
	RegisterTestingT(t)

	ctx := context.Background()
	eventsDao := mocks.NewEventDao()
	resourcesDao := mocks.NewResourceDao()
	events := services.NewEventService(eventsDao)
	eventServer := &exampleEventServer{eventsDao: eventsDao, resourcesDao: resourcesDao, subscrbers: []string{"cluster1"}}
	mgr := NewKindControllerManager(NewPredicatedEventFilter(eventServer.PredicateEvent), events)

	ctrl := &exampleController{}
	config := newExampleControllerConfig(ctrl)
	mgr.Add(config)

	resID := uuid.New().String()
	_, err := resourcesDao.Create(ctx, &api.Resource{
		Meta:         api.Meta{ID: resID},
		ConsumerName: "cluster1",
		Source:       config.Source,
	})
	Expect(err).To(BeNil())

	_, err = eventsDao.Create(ctx, &api.Event{
		Meta:      api.Meta{ID: "1"},
		Source:    config.Source,
		SourceID:  resID,
		EventType: api.CreateEventType,
	})
	Expect(err).To(BeNil())

	_, err = eventsDao.Create(ctx, &api.Event{
		Meta:      api.Meta{ID: "2"},
		Source:    config.Source,
		SourceID:  "any id",
		EventType: api.UpdateEventType,
	})
	Expect(err).To(BeNil())

	newResID := uuid.New().String()
	_, err = resourcesDao.Create(ctx, &api.Resource{
		Meta:         api.Meta{ID: newResID},
		ConsumerName: "cluster2",
		Source:       config.Source,
	})
	Expect(err).To(BeNil())

	_, err = eventsDao.Create(ctx, &api.Event{
		Meta:      api.Meta{ID: "3"},
		Source:    config.Source,
		SourceID:  newResID,
		EventType: api.DeleteEventType,
	})
	Expect(err).To(BeNil())

	mgr.handleEvent(ctx, "1")
	mgr.handleEvent(ctx, "2")
	mgr.handleEvent(ctx, "3")

	Expect(ctrl.addCounter).To(Equal(1))
	Expect(ctrl.updateCounter).To(Equal(0))
	Expect(ctrl.deleteCounter).To(Equal(0))

	eve, err := eventsDao.Get(ctx, "1")
	Expect(err).To(BeNil())
	Expect(eve.ReconciledDate).ToNot(BeNil(), "event reconcile date should be set")

	eve, err = eventsDao.Get(ctx, "2")
	Expect(err).To(BeNil())
	Expect(eve.ReconciledDate).ToNot(BeNil(), "event reconcile date should be set")

	eve, err = eventsDao.Get(ctx, "3")
	Expect(err).To(BeNil())
	Expect(eve.ReconciledDate).To(BeNil(), "event reconcile date should not be set")
}
