package integration

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
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

	// start work agent so that grpc broker can work
	consumer := h.CreateConsumer("cluster-" + rand.String(5))
	h.StartWorkAgent(ctx, consumer.Name)

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

	// Start 3 controllers concurrently for message queue event server
	threads := 3
	randNum := rand.Intn(3)
	for i := 0; i < threads; i++ {
		// each controller has its own event filter, otherwise, the event lock will block the event processing.
		eventFilter := controllers.NewLockBasedEventFilter(db.NewAdvisoryLockFactory(h.Env().Database.SessionFactory))
		if h.Broker == "grpc" {
			eventFilter = controllers.NewPredicatedEventFilter(func(ctx context.Context, eventID string) (bool, error) {
				// simulate the event filter, where the agent randomly connects to a grpc broker instance.
				// in theory, only one broker instance should process the event.
				return i == randNum, nil
			})
		}
		go func() {
			s := &server.ControllersServer{
				KindControllerManager: controllers.NewKindControllerManager(
					eventFilter,
					h.Env().Services.Events(),
				),
				StatusController: controllers.NewStatusController(
					h.Env().Services.StatusEvents(),
					dao.NewInstanceDao(&h.Env().Database.SessionFactory),
					dao.NewEventInstanceDao(&h.Env().Database.SessionFactory),
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

	// start work agent so that grpc broker can work
	consumer := h.CreateConsumer("cluster-" + rand.String(5))
	h.StartWorkAgent(ctx, consumer.Name)

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
				h.EventFilter,
				h.Env().Services.Events(),
			),
			StatusController: controllers.NewStatusController(
				h.Env().Services.StatusEvents(),
				dao.NewInstanceDao(&h.Env().Database.SessionFactory),
				dao.NewEventInstanceDao(&h.Env().Database.SessionFactory),
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
	time.Sleep(time.Second)

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

	// start work agent so that grpc broker can work
	consumer := h.CreateConsumer("cluster-" + rand.String(5))
	h.StartWorkAgent(ctx, consumer.Name)

	// create two resources with resource dao
	resource4ID := uuid.New().String()
	resourceDao := dao.NewResourceDao(&h.Env().Database.SessionFactory)
	if _, err := resourceDao.Create(ctx, &api.Resource{
		Meta: api.Meta{
			ID: resource4ID,
		},
		ConsumerName: consumer.Name,
		Name:         "resource4",
	}); err != nil {
		t.Fatal(err)
	}

	resource5ID := uuid.New().String()
	if _, err := resourceDao.Create(ctx, &api.Resource{
		Meta: api.Meta{
			ID: resource5ID,
		},
		ConsumerName: consumer.Name,
		Name:         "resource5",
	}); err != nil {
		t.Fatal(err)
	}

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
		SourceID:  resource4ID,
		EventType: api.UpdateEventType}); err != nil {
		t.Fatal(err)
	}
	if _, err := eventDao.Create(ctx, &api.Event{Source: "Resources",
		SourceID:  resource5ID,
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
				h.EventFilter,
				h.Env().Services.Events(),
			),
			StatusController: controllers.NewStatusController(
				h.Env().Services.StatusEvents(),
				dao.NewInstanceDao(&h.Env().Database.SessionFactory),
				dao.NewEventInstanceDao(&h.Env().Database.SessionFactory),
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

func TestStatusControllerSync(t *testing.T) {
	h, _ := test.RegisterIntegration(t)

	account := h.NewRandAccount()
	ctx, cancel := context.WithCancel(h.NewAuthenticatedContext(account))

	instanceDao := dao.NewInstanceDao(&h.Env().Database.SessionFactory)
	statusEventDao := dao.NewStatusEventDao(&h.Env().Database.SessionFactory)
	eventInstanceDao := dao.NewEventInstanceDao(&h.Env().Database.SessionFactory)

	// prepare instances
	if _, err := instanceDao.Create(ctx, &api.ServerInstance{
		Meta: api.Meta{ID: "i1"}, Ready: true, LastHeartbeat: time.Now()}); err != nil {
		t.Fatal(err)
	}
	if _, err := instanceDao.Create(ctx, &api.ServerInstance{Meta: api.Meta{ID: "i2"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := instanceDao.Create(ctx, &api.ServerInstance{
		Meta: api.Meta{ID: "i3"}, Ready: true, LastHeartbeat: time.Now()}); err != nil {
		t.Fatal(err)
	}

	// prepare events
	evt1, err := statusEventDao.Create(ctx, &api.StatusEvent{})
	if err != nil {
		t.Fatal(err)
	}
	evt2, err := statusEventDao.Create(ctx, &api.StatusEvent{})
	if err != nil {
		t.Fatal(err)
	}
	evt3, err := statusEventDao.Create(ctx, &api.StatusEvent{})
	if err != nil {
		t.Fatal(err)
	}
	evt4, err := statusEventDao.Create(ctx, &api.StatusEvent{})
	if err != nil {
		t.Fatal(err)
	}
	evt5, err := statusEventDao.Create(ctx, &api.StatusEvent{})
	if err != nil {
		t.Fatal(err)
	}

	readyInstances, err := instanceDao.FindReadyIDs(ctx)
	if err != nil {
		t.Fatal(err)
	}

	// prepare event-instances
	for _, id := range readyInstances {
		if _, err := eventInstanceDao.Create(ctx, &api.EventInstance{InstanceID: id, EventID: evt1.ID}); err != nil {
			t.Fatal(err)
		}
		if _, err := eventInstanceDao.Create(ctx, &api.EventInstance{InstanceID: id, EventID: evt2.ID}); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := eventInstanceDao.Create(ctx, &api.EventInstance{InstanceID: "i2", EventID: evt1.ID}); err != nil {
		t.Fatal(err)
	}
	if _, err := eventInstanceDao.Create(ctx, &api.EventInstance{InstanceID: "i1", EventID: evt3.ID}); err != nil {
		t.Fatal(err)
	}
	if _, err := eventInstanceDao.Create(ctx, &api.EventInstance{InstanceID: "i2", EventID: evt3.ID}); err != nil {
		t.Fatal(err)
	}
	if _, err := eventInstanceDao.Create(ctx, &api.EventInstance{InstanceID: "i1", EventID: evt4.ID}); err != nil {
		t.Fatal(err)
	}
	if _, err := eventInstanceDao.Create(ctx, &api.EventInstance{InstanceID: "i3", EventID: evt5.ID}); err != nil {
		t.Fatal(err)
	}

	// start the controller
	go func() {
		s := &server.ControllersServer{
			KindControllerManager: controllers.NewKindControllerManager(
				h.EventFilter,
				h.Env().Services.Events(),
			),
			StatusController: controllers.NewStatusController(
				h.Env().Services.StatusEvents(),
				dao.NewInstanceDao(&h.Env().Database.SessionFactory),
				dao.NewEventInstanceDao(&h.Env().Database.SessionFactory),
			),
		}

		s.Start(ctx)
	}()

	purged := []string{evt1.ID, evt2.ID}
	remained := []string{evt3.ID, evt4.ID, evt5.ID}
	Eventually(func() error {
		events, err := statusEventDao.FindByIDs(ctx, remained)
		if err != nil {
			return err
		}

		if len(events) != 3 {
			return fmt.Errorf("should have events %s remained, but got %v", remained, events)
		}

		events, err = statusEventDao.FindByIDs(ctx, purged)
		if err != nil {
			return err
		}

		if len(events) != 0 {
			return fmt.Errorf("should purge the events %s, but got %+v", purged, events)
		}

		eventInstances, err := eventInstanceDao.FindStatusEvents(ctx, purged)
		if err != nil {
			return err
		}
		if len(eventInstances) != 0 {
			return fmt.Errorf("should purge the event-instances %s, but got %+v", purged, eventInstances)
		}

		if _, err := eventInstanceDao.Get(ctx, evt3.ID, "i1"); err != nil {
			return fmt.Errorf("%s-%s is not found", "e3", "i1")
		}
		if _, err := eventInstanceDao.Get(ctx, evt3.ID, "i2"); err != nil {
			return fmt.Errorf("%s-%s is not found", "e3", "i2")
		}
		if _, err := eventInstanceDao.Get(ctx, evt4.ID, "i1"); err != nil {
			return fmt.Errorf("%s-%s is not found", "e4", "i1")
		}
		if _, err := eventInstanceDao.Get(ctx, evt5.ID, "i3"); err != nil {
			return fmt.Errorf("%s-%s is not found", "e5", "i3")
		}

		return nil
	}, 5*time.Second, 1*time.Second).Should(Succeed())

	// cleanup
	for _, evtID := range remained {
		if err := statusEventDao.Delete(ctx, evtID); err != nil {
			t.Fatal(err)
		}
	}
	if err := instanceDao.DeleteByIDs(ctx, []string{"i1", "i2", "i3"}); err != nil {
		t.Fatal(err)
	}

	// cancel the context to stop the controller manager
	cancel()
}
