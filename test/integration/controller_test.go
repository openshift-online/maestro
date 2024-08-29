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
	"k8s.io/apimachinery/pkg/util/rand"
)

func TestControllerRacing(t *testing.T) {
	h, _ := test.RegisterIntegration(t)

	account := h.NewRandAccount()
	ctx, cancel := context.WithCancel(h.NewAuthenticatedContext(account))
	defer func() {
		cancel()
	}()

	eventDao := dao.NewEventDao(&h.Env().Database.SessionFactory)
	statusEventDao := dao.NewStatusEventDao(&h.Env().Database.SessionFactory)

	// The handler filters the events by source id/type/reconciled, and only record
	// the event with create type. Due to the event lock, each create event
	// should be only processed once.
	var proccessedEvent, processedStatusEvent []string
	onUpsert := func(ctx context.Context, id string) error {
		events, err := eventDao.All(ctx)
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

	onStatusUpdate := func(ctx context.Context, eventID, resourceID string) error {
		statusEvents, err := statusEventDao.All(ctx)
		if err != nil {
			return err
		}

		for _, evt := range statusEvents {
			if evt.ID != eventID || evt.ResourceID != resourceID {
				continue
			}
			// the event has been reconciled by others, ignore.
			if evt.ReconciledDate != nil {
				continue
			}
			processedStatusEvent = append(processedStatusEvent, eventID)
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
				StatusController: controllers.NewStatusController(
					h.Env().Services.StatusEvents(),
				),
			}

			s.KindControllerManager.Add(&controllers.ControllerConfig{
				Source: "Resources",
				Handlers: map[api.EventType][]controllers.ControllerHandlerFunc{
					api.CreateEventType: {onUpsert},
				},
			})

			s.StatusController.Add(map[api.StatusEventType][]controllers.StatusHandlerFunc{
				api.StatusUpdateEventType: {onStatusUpdate},
			})

			s.Start(ctx)
		}()
	}
	// wait for controller service starts
	time.Sleep(3 * time.Second)

	consumer := h.CreateConsumer("cluster-" + rand.String(5))
	resources := h.CreateResourceList(consumer.Name, 50)

	// This is to check only 50 create events are processed. It waits for 5 seconds to ensure all events have been
	// processed by the controllers.
	Eventually(func() error {
		if len(proccessedEvent) != 50 {
			return fmt.Errorf("should have 50 create events but got %d", len(proccessedEvent))
		}
		return nil
	}, 10*time.Second, 1*time.Second).Should(Succeed())

	// create 50 update status events
	for _, resource := range resources {
		_, sErr := statusEventDao.Create(ctx, &api.StatusEvent{
			ResourceID:      resource.ID,
			StatusEventType: api.StatusUpdateEventType,
		})
		if sErr != nil {
			t.Fatalf("failed to create status event: %v", sErr)
		}
		time.Sleep(10 * time.Millisecond)
	}

	// This is to check 150 status update events are processed. It waits for 10 seconds to ensure all status events have been
	// processed by the controllers.
	Eventually(func() error {
		if len(processedStatusEvent) != threads*50 {
			return fmt.Errorf("should have 150 update status events but got %d", len(processedStatusEvent))
		}
		return nil
	}, 10*time.Second, 1*time.Second).Should(Succeed())
}

func TestControllerReconcile(t *testing.T) {
	h, _ := test.RegisterIntegration(t)

	account := h.NewRandAccount()
	ctx, cancel := context.WithCancel(h.NewAuthenticatedContext(account))

	eventDao := dao.NewEventDao(&h.Env().Database.SessionFactory)
	statusEventDao := dao.NewStatusEventDao(&h.Env().Database.SessionFactory)

	processedEventTimes := 0
	// this handler will return an error at the first time to simulate an error happened when handing an event,
	// and then, the controller will requeue this event, at that time, we handle this event successfully.
	onUpsert := func(ctx context.Context, id string) error {
		processedEventTimes = processedEventTimes + 1
		if processedEventTimes == 1 {
			return fmt.Errorf("failed to process the event")
		}

		return nil
	}

	processedStatusEventTimes := 0
	// this handler will return an error at the first time to simulate an error happened when handing an event,
	// and then, the controller will requeue this event, at that time, we handle this event successfully.
	onStatusUpdate := func(ctx context.Context, eventID, resourceID string) error {
		processedStatusEventTimes = processedStatusEventTimes + 1
		if processedStatusEventTimes == 1 {
			return fmt.Errorf("failed to process the status event")
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
			StatusController: controllers.NewStatusController(
				h.Env().Services.StatusEvents(),
			),
		}

		s.KindControllerManager.Add(&controllers.ControllerConfig{
			Source: "Resources",
			Handlers: map[api.EventType][]controllers.ControllerHandlerFunc{
				api.CreateEventType: {onUpsert},
				api.UpdateEventType: {onUpsert},
			},
		})
		s.StatusController.Add(map[api.StatusEventType][]controllers.StatusHandlerFunc{
			api.StatusUpdateEventType: {onStatusUpdate},
			api.StatusDeleteEventType: {onStatusUpdate},
		})

		s.Start(ctx)
	}()
	// wait for the listener to start
	time.Sleep(100 * time.Millisecond)

	consumer := h.CreateConsumer("cluster-" + rand.String(5))
	deployName := fmt.Sprintf("nginx-%s", rand.String(5))
	resource := h.CreateResource(consumer.Name, deployName, 1)

	// Eventually, the event will be processed by the controller.
	Eventually(func() error {
		if processedEventTimes != 2 {
			return fmt.Errorf("the event should be processed 2 times, but got %d", processedEventTimes)
		}

		events, err := eventDao.All(ctx)
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

	// create pdate status event
	_, sErr := statusEventDao.Create(ctx, &api.StatusEvent{
		ResourceID:      resource.ID,
		StatusEventType: api.StatusUpdateEventType,
	})
	if sErr != nil {
		t.Fatalf("failed to create status event: %v", sErr)
	}

	// Eventually, the status event will be processed by the controller.
	Eventually(func() error {
		if processedStatusEventTimes != 2 {
			return fmt.Errorf("the status event should be processed 2 times, but got %d", processedStatusEventTimes)
		}

		statusEvents, err := statusEventDao.All(ctx)
		if err != nil {
			return err
		}
		if len(statusEvents) != 1 {
			return fmt.Errorf("too many status events %d", len(statusEvents))
		}
		// if statusEvents[0].ReconciledDate == nil {
		// 	return fmt.Errorf("the event should be reconciled")
		// }
		return nil
	}, 5*time.Second, 1*time.Second).Should(Succeed())

	// cancel the context to stop the controller manager
	cancel()
}

func TestControllerSync(t *testing.T) {
	h, _ := test.RegisterIntegration(t)

	account := h.NewRandAccount()
	ctx, cancel := context.WithCancel(h.NewAuthenticatedContext(account))

	eventDao := dao.NewEventDao(&h.Env().Database.SessionFactory)

	now := time.Now()
	if _, err := eventDao.Create(ctx, &api.Event{Source: "Resources",
		SourceID:       "resource1",
		EventType:      api.UpdateEventType,
		ReconciledDate: &now}); err != nil {
		t.Fatal(err)
	}
	if _, err := eventDao.Create(ctx, &api.Event{Source: "Resources",
		SourceID:       "resource2",
		EventType:      api.UpdateEventType,
		ReconciledDate: &now}); err != nil {
		t.Fatal(err)
	}
	if _, err := eventDao.Create(ctx, &api.Event{Source: "Resources",
		SourceID:       "resource3",
		EventType:      api.UpdateEventType,
		ReconciledDate: &now}); err != nil {
		t.Fatal(err)
	}
	if _, err := eventDao.Create(ctx, &api.Event{Source: "Resources",
		SourceID:  "resource4",
		EventType: api.UpdateEventType}); err != nil {
		t.Fatal(err)
	}
	if _, err := eventDao.Create(ctx, &api.Event{Source: "Resources",
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
			StatusController: controllers.NewStatusController(
				h.Env().Services.StatusEvents(),
			),
		}

		s.KindControllerManager.Add(&controllers.ControllerConfig{
			Source: "Resources",
			Handlers: map[api.EventType][]controllers.ControllerHandlerFunc{
				api.CreateEventType: {onUpsert},
				api.UpdateEventType: {onUpsert},
			},
		})

		s.Start(ctx)
	}()

	// Eventually, the controller should only handle the one unreconciled event.
	Eventually(func() error {
		if len(proccessedEvents) != 2 {
			return fmt.Errorf("should have only two unreconciled events but got %d", len(proccessedEvents))
		}

		events, err := eventDao.All(ctx)
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
