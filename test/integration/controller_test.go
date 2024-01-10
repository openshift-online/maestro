package integration

import (
	"context"
	"fmt"
	"testing"
	"time"

	. "github.com/onsi/gomega"
	"github.com/openshift-online/maestro/cmd/maestro/server"
	"github.com/openshift-online/maestro/pkg/api"
	"github.com/openshift-online/maestro/pkg/controllers"
	"github.com/openshift-online/maestro/pkg/dao"
	"github.com/openshift-online/maestro/pkg/db"
	"github.com/openshift-online/maestro/test"
)

func TestControllerRacing(t *testing.T) {
	h, _ := test.RegisterIntegration(t)

	account := h.NewRandAccount()
	ctx, cancel := context.WithCancel(h.NewAuthenticatedContext(account))

	dao := dao.NewEventDao(&h.Env().Database.SessionFactory)

	// The handler filters the events by source id/type/reconciled, and only record
	// the event with create type. Due to the event lock, each create event
	// should be only processed once.
	var proccessedEvent []string
	onUpsert := func(ctx context.Context, id string) error {
		events, err := dao.All(ctx)
		if err != nil {
			return err
		}

		for _, evt := range events {
			if evt.SourceID != id {
				continue
			}
			if evt.EventType != api.CreateEventType {
				continue
			}
			// the event has been reconciled by others, ignore.
			if evt.ReconciledDate != nil {
				continue
			}
			proccessedEvent = append(proccessedEvent, id)
		}

		return nil
	}

	// Start 3 controllers concurrently
	threads := 3
	for i := 0; i < threads; i++ {
		go func() {
			s := &server.ControllersServer{
				KindControllerManager: controllers.NewKindControllerManager(
					db.NewAdvisoryLockFactory(h.Env().Database.SessionFactory),
					h.Env().Services.Events(),
				),
			}

			s.KindControllerManager.Add(&controllers.ControllerConfig{
				Source: "Resources",
				Handlers: map[api.EventType][]controllers.ControllerHandlerFunc{
					api.CreateEventType: {onUpsert},
					api.UpdateEventType: {onUpsert},
				},
			})

			s.Start(ctx.Done())
		}()
	}

	consumer := h.NewConsumer("cluster1")
	_ = h.NewResourceList(consumer.ID, 50)

	// This is to check only 50 create events is processed. It waits for 5 seconds to ensure all events have been
	// processed by the controllers.
	Eventually(func() error {
		if len(proccessedEvent) != 50 {
			return fmt.Errorf("should have only 50 create events but got %d", len(proccessedEvent))
		}
		return nil
	}, 5*time.Second, 1*time.Second).Should(Succeed())

	// cancel the context to stop the controller manager
	cancel()
}

func TestControllerReconcile(t *testing.T) {
	h, _ := test.RegisterIntegration(t)

	account := h.NewRandAccount()
	ctx, cancel := context.WithCancel(h.NewAuthenticatedContext(account))

	dao := dao.NewEventDao(&h.Env().Database.SessionFactory)

	processedTimes := 0
	// this handler will return an error at the first time to simulate an error happened when handing an event,
	// and then, the controller will requeue this event, at that time, we handle this event successfully.
	onUpsert := func(ctx context.Context, id string) error {
		processedTimes = processedTimes + 1
		if processedTimes == 1 {
			return fmt.Errorf("failed to process the event")
		}

		return nil
	}

	// start controller to handle events
	go func() {
		s := &server.ControllersServer{
			KindControllerManager: controllers.NewKindControllerManager(
				db.NewAdvisoryLockFactory(h.Env().Database.SessionFactory),
				h.Env().Services.Events(),
			),
		}

		s.KindControllerManager.Add(&controllers.ControllerConfig{
			Source: "Resources",
			Handlers: map[api.EventType][]controllers.ControllerHandlerFunc{
				api.CreateEventType: {onUpsert},
				api.UpdateEventType: {onUpsert},
			},
		})

		s.Start(ctx.Done())
	}()

	consumer := h.NewConsumer("cluster1")
	_ = h.NewResource(consumer.ID, 1)

	// Eventually, the event will be processed by the controller.
	Eventually(func() error {
		if processedTimes != 2 {
			return fmt.Errorf("the event should be processed 2 times, but got %d", processedTimes)
		}

		events, err := dao.All(ctx)
		if err != nil {
			return err
		}
		if len(events) != 1 {
			return fmt.Errorf("too many events %d", len(events))
		}
		if events[0].ReconciledDate == nil {
			return fmt.Errorf("the event should be reconciled")
		}
		return nil
	}, 5*time.Second, 1*time.Second).Should(Succeed())

	// cancel the context to stop the controller manager
	cancel()
}

func TestControllerSync(t *testing.T) {
	h, _ := test.RegisterIntegration(t)

	account := h.NewRandAccount()
	ctx, cancel := context.WithCancel(h.NewAuthenticatedContext(account))

	dao := dao.NewEventDao(&h.Env().Database.SessionFactory)

	now := time.Now()
	if _, err := dao.Create(ctx, &api.Event{Source: "Resources",
		SourceID:       "resource1",
		EventType:      api.UpdateEventType,
		ReconciledDate: &now}); err != nil {
		t.Fatal(err)
	}
	if _, err := dao.Create(ctx, &api.Event{Source: "Resources",
		SourceID:       "resource2",
		EventType:      api.UpdateEventType,
		ReconciledDate: &now}); err != nil {
		t.Fatal(err)
	}
	if _, err := dao.Create(ctx, &api.Event{Source: "Resources",
		SourceID:       "resource3",
		EventType:      api.UpdateEventType,
		ReconciledDate: &now}); err != nil {
		t.Fatal(err)
	}
	if _, err := dao.Create(ctx, &api.Event{Source: "Resources",
		SourceID:  "resource4",
		EventType: api.UpdateEventType}); err != nil {
		t.Fatal(err)
	}
	if _, err := dao.Create(ctx, &api.Event{Source: "Resources",
		SourceID:  "resource5",
		EventType: api.UpdateEventType}); err != nil {
		t.Fatal(err)
	}

	var proccessedEvents []string
	onUpsert := func(ctx context.Context, id string) error {
		// we just record the processed event
		proccessedEvents = append(proccessedEvents, id)
		return nil
	}

	// start the controller, once the controller started, it will sync the events:
	// - clean up the reconciled events
	// - requeue the unreconciled events
	go func() {
		s := &server.ControllersServer{
			KindControllerManager: controllers.NewKindControllerManager(
				db.NewAdvisoryLockFactory(h.Env().Database.SessionFactory),
				h.Env().Services.Events(),
			),
		}

		s.KindControllerManager.Add(&controllers.ControllerConfig{
			Source: "Resources",
			Handlers: map[api.EventType][]controllers.ControllerHandlerFunc{
				api.CreateEventType: {onUpsert},
				api.UpdateEventType: {onUpsert},
			},
		})

		s.Start(ctx.Done())
	}()

	// Eventually, the controller should only handle the one unreconciled event.
	Eventually(func() error {
		if len(proccessedEvents) != 2 {
			return fmt.Errorf("should have only two unreconciled events but got %d", len(proccessedEvents))
		}

		events, err := dao.All(ctx)
		if err != nil {
			return err
		}

		if len(events) != 2 {
			return fmt.Errorf("should have only two events remained but got %d", len(events))
		}

		return nil
	}, 5*time.Second, 1*time.Second).Should(Succeed())

	// cancel the context to stop the controller manager
	cancel()
}
