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

func TestControllerFramework(t *testing.T) {
	RegisterTestingT(t)

	ctx := context.Background()
	eventsDao := mocks.NewEventDao()
	events := services.NewEventService(eventsDao)
	mgr := NewKindControllerManager(dbmocks.NewMockAdvisoryLockFactory(), events)

	ctrl := &exampleController{}
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
}
