package integration

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	. "github.com/onsi/gomega"
	"github.com/prometheus/client_golang/prometheus"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"k8s.io/apimachinery/pkg/util/rand"

	"github.com/openshift-online/maestro/cmd/maestro/server"
	"github.com/openshift-online/maestro/pkg/api"
	"github.com/openshift-online/maestro/pkg/controllers"
	"github.com/openshift-online/maestro/pkg/dao"
	"github.com/openshift-online/maestro/pkg/db"
	"github.com/openshift-online/maestro/test"
)

func TestControllerRacing(t *testing.T) {
	h, _ := test.RegisterIntegration(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer func() {
		cancel()
	}()

	// start work agent so that grpc broker can work
	consumer, err := h.CreateConsumer("cluster-" + rand.String(5))
	Expect(err).NotTo(HaveOccurred())
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

	resources, err := h.CreateResourceList(consumer.Name, 50)
	Expect(err).NotTo(HaveOccurred())

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
	ctx, cancel := context.WithCancel(context.Background())

	// start work agent so that grpc broker can work
	consumer, err := h.CreateConsumer("cluster-" + rand.String(5))
	Expect(err).NotTo(HaveOccurred())
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
	resource, err := h.CreateResource(uuid.NewString(), consumer.Name, deployName, "default", 1)
	Expect(err).NotTo(HaveOccurred())

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
	ctx, cancel := context.WithCancel(context.Background())

	// start work agent so that grpc broker can work
	consumer, err := h.CreateConsumer("cluster-" + rand.String(5))
	Expect(err).NotTo(HaveOccurred())
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
	ctx, cancel := context.WithCancel(context.Background())

	instanceDao := dao.NewInstanceDao(&h.Env().Database.SessionFactory)
	statusEventDao := dao.NewStatusEventDao(&h.Env().Database.SessionFactory)
	eventInstanceDao := dao.NewEventInstanceDao(&h.Env().Database.SessionFactory)

	// prepare instances
	// Wrap the check and create in a transaction to avoid race conditions
	err := h.Env().Database.SessionFactory.New(ctx).Transaction(func(tx *gorm.DB) error {
		var instance api.ServerInstance
		result := tx.Where("id = ?", "maestro").First(&instance)

		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			// create a new instance if not found
			newInstance := &api.ServerInstance{
				Meta:          api.Meta{ID: "maestro"},
				Ready:         true,
				LastHeartbeat: time.Now(),
			}
			// best-effort insert; ignore if another tx inserts first
			if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(newInstance).Error; err != nil {
				return err
			}
		} else if result.Error != nil {
			return result.Error
		}

		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
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

func TestMultipleControllers(t *testing.T) {
	h, _ := test.RegisterIntegration(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer func() {
		cancel()
	}()

	consumer, err := h.CreateConsumer("cluster-" + rand.String(5))
	Expect(err).NotTo(HaveOccurred())

	statusCtrl := controllers.NewStatusController(
		h.Env().Services.StatusEvents(),
		dao.NewInstanceDao(&h.Env().Database.SessionFactory),
		dao.NewEventInstanceDao(&h.Env().Database.SessionFactory),
	)
	statusCtrl.Add(map[api.StatusEventType][]controllers.StatusHandlerFunc{
		api.StatusUpdateEventType: {func(ctx context.Context, eventID, sourceID string) error { return nil }},
	})

	handledByCtrl0 := false

	ctrl0 := controllers.NewKindControllerManager(
		controllers.NewLockBasedEventFilter(db.NewAdvisoryLockFactory(h.Env().Database.SessionFactory)),
		h.Env().Services.Events(),
	)
	ctrl0.Add(&controllers.ControllerConfig{
		Source: "Resources",
		Handlers: map[api.EventType][]controllers.ControllerHandlerFunc{
			api.CreateEventType: {func(ctx context.Context, id string) error {
				handledByCtrl0 = true
				return nil
			}},
		},
	})
	ctrl1 := controllers.NewKindControllerManager(
		controllers.NewLockBasedEventFilter(db.NewAdvisoryLockFactory(h.Env().Database.SessionFactory)),
		h.Env().Services.Events(),
	)
	ctrl1.Add(&controllers.ControllerConfig{
		Source: "Resources",
		Handlers: map[api.EventType][]controllers.ControllerHandlerFunc{
			api.CreateEventType: {func(ctx context.Context, id string) error { return fmt.Errorf("cannot handle resource") }},
		},
	})
	ctrl2 := controllers.NewKindControllerManager(
		controllers.NewLockBasedEventFilter(db.NewAdvisoryLockFactory(h.Env().Database.SessionFactory)),
		h.Env().Services.Events(),
	)
	ctrl2.Add(&controllers.ControllerConfig{
		Source: "Resources",
		Handlers: map[api.EventType][]controllers.ControllerHandlerFunc{
			api.CreateEventType: {func(ctx context.Context, id string) error { return fmt.Errorf("cannot handle resource") }},
		},
	})

	go func() {
		s := &server.ControllersServer{
			KindControllerManager: ctrl0,
			StatusController:      statusCtrl,
		}
		s.Start(ctx)
	}()
	go func() {
		s := &server.ControllersServer{
			KindControllerManager: ctrl1,
			StatusController:      statusCtrl,
		}
		s.Start(ctx)
	}()
	go func() {
		s := &server.ControllersServer{
			KindControllerManager: ctrl2,
			StatusController:      statusCtrl,
		}
		s.Start(ctx)
	}()

	// wait for controller service starts
	time.Sleep(3 * time.Second)

	_, err = h.CreateResourceList(consumer.Name, 1)
	Expect(err).NotTo(HaveOccurred())

	Eventually(func() error {
		if !handledByCtrl0 {
			return fmt.Errorf("expected handled by controller 0, but failed")
		}
		if ctrl0.Queue().Len() != 0 {
			return fmt.Errorf("expected queue len is 0 in controller 0, but failed")
		}
		if ctrl1.Queue().Len() != 0 {
			return fmt.Errorf("expected queue len is 0 in controller 1, but failed")
		}
		if ctrl2.Queue().Len() != 0 {
			return fmt.Errorf("expected queue len is 0 in controller 2, but failed")
		}
		return nil
	}, 10*time.Second, 1*time.Second).Should(Succeed())

}

func TestSpecEventAgeMetric(t *testing.T) {
	h, _ := test.RegisterIntegration(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create an unreconciled event backdated two minutes
	evt := &api.Event{
		Meta: api.Meta{
			CreatedAt: time.Now().Add(-2 * time.Minute),
		},
		Source:    "Resources",
		SourceID:  uuid.NewString(),
		EventType: api.CreateEventType,
	}
	err := h.Env().Database.SessionFactory.New(ctx).Transaction(func(tx *gorm.DB) error {
		return tx.Omit(clause.Associations).Create(evt).Error
	})
	if err != nil {
		t.Fatal(err)
	}
	eventDao := dao.NewEventDao(&h.Env().Database.SessionFactory)
	evt, err = eventDao.Get(ctx, evt.ID)
	if err != nil {
		t.Fatal(err)
	}
	Expect(time.Since(evt.CreatedAt).Seconds()).Should(BeNumerically(">=", 120))

	// Start the controller. No handlers are registered so the event will remain unreconciled
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

	// reportOldestEvent fires immediately on startup, so the age metric should be
	// populated promptly
	var age float64
	Eventually(func() error {
		families, err := prometheus.DefaultGatherer.Gather()
		if err != nil {
			return err
		}
		for _, mf := range families {
			if mf.GetName() == "spec_controller_event_oldest_unreconciled_age_seconds" {
				metrics := mf.GetMetric()
				if len(metrics) == 0 {
					return fmt.Errorf("metric has no samples yet")
				}
				age = metrics[0].GetGauge().GetValue()
				if age == 0 {
					return fmt.Errorf("metric sample is zero")
				}
				return nil
			}
		}
		return fmt.Errorf("metric spec_controller_event_oldest_unreconciled_age_seconds not found")
	}, 10*time.Second, 1*time.Second).Should(Succeed())
	Expect(age).Should(BeNumerically(">=", 120))
}

func TestNotificationQueueUsageMetric(t *testing.T) {
	h, _ := test.RegisterIntegration(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start a slow listener that deliberately delays notification consumption,
	// causing notifications to accumulate in the postgres notification queue.
	channel := "test_queue_usage_" + rand.String(5)
	h.Env().Database.SessionFactory.NewListener(ctx, channel, func(id string) {
		time.Sleep(100 * time.Second)
	})

	// Send NOTIFY's in a loop from a separate goroutine to fill the queue.
	// The slow listener can only drain one notification every 10s, so the
	// postgres NOTIFY queue usage will increase.
	payload := strings.Repeat("x", 7000)
	var mu sync.Mutex
	var lastNotifyErr error
	go func() {
		notifyDB := h.Env().Database.SessionFactory.New(ctx)
		for {
			select {
			case <-ctx.Done():
				return
			default:
				err := notifyDB.Exec("SELECT pg_notify(?, ?)", channel, payload).Error
				mu.Lock()
				lastNotifyErr = err
				mu.Unlock()
			}
			time.Sleep(time.Millisecond)
		}
	}()

	// Allow time for the notification queue to build up
	time.Sleep(2 * time.Second)

	// Start the StatusController, which calls reportNotificationQueueUsage on start
	consumer, err := h.CreateConsumer("cluster-" + rand.String(5))
	Expect(err).NotTo(HaveOccurred())
	h.StartWorkAgent(ctx, consumer.Name)

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

	// Wait for the metric to be reported with a value > 0, indicating
	// that the notification queue has accumulated undelivered notifications.
	metricName := "postgres_notification_queue_usage"
	Eventually(func() error {
		gathered, gatherErr := prometheus.DefaultGatherer.Gather()
		if gatherErr != nil {
			return gatherErr
		}
		for _, mf := range gathered {
			if *mf.Name == metricName {
				if len(mf.Metric) == 0 {
					return fmt.Errorf("metric %s has no samples", metricName)
				}
				usage := mf.Metric[0].Gauge.GetValue()
				if usage <= 0 {
					return fmt.Errorf("metric %s value %f is not > 0", metricName, usage)
				}
				return nil
			}
		}
		return fmt.Errorf("metric %s not found", metricName)
	}, 10*time.Second, 1*time.Second).Should(Succeed(), func() string {
		mu.Lock()
		defer mu.Unlock()
		return fmt.Sprintf("Last NOTIFY error: %v", lastNotifyErr)
	})
}
