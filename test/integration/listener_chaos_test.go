package integration

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/util/rand"

	"github.com/openshift-online/maestro/pkg/api"
	"github.com/openshift-online/maestro/pkg/controllers"
	"github.com/openshift-online/maestro/test"
)

// TestListenerDropRecovery verifies that events created while the pg_notify
// listener is down are still picked up by the syncEventsIfQueueIsEmpty path.
//
// It uses separate contexts for the controller and listener so the listener
// can be cleanly stopped while the controller keeps running. Events created
// after the listener stops must be discovered by syncEventsIfQueueIsEmpty.
func TestListenerDropRecovery(t *testing.T) {
	h, _ := test.RegisterIntegration(t)

	controllerCtx, cancelController := context.WithCancel(context.Background())
	defer cancelController()

	consumer, err := h.CreateConsumer("cluster-" + rand.String(5))
	Expect(err).NotTo(HaveOccurred())
	h.StartWorkAgent(controllerCtx, consumer.Name)

	var mu sync.Mutex
	processedTimes := map[string]time.Time{}

	onUpsert := func(ctx context.Context, id string) error {
		mu.Lock()
		defer mu.Unlock()
		if _, seen := processedTimes[id]; !seen {
			processedTimes[id] = time.Now()
		}
		return nil
	}

	h.Env().Database.SessionFactory.New(controllerCtx).Exec("DELETE FROM events")

	// Use a pass-through event filter. No advisory locks needed since we
	// control the only controller instance processing these events.
	// This avoids lock contention with stale controllers from previous tests
	// and predicate errors in gRPC mode on missing events.
	km := controllers.NewKindControllerManager(
		controllers.NewPredicatedEventFilter(func(ctx context.Context, eventID string) (bool, error) {
			return true, nil
		}),
		h.Env().Services.Events(),
	)
	km.Add(&controllers.ControllerConfig{
		Source: "Resources",
		Handlers: map[api.EventType][]controllers.ControllerHandlerFunc{
			api.CreateEventType: {onUpsert},
			api.UpdateEventType: {onUpsert},
		},
	})

	go km.Run(controllerCtx)

	// Start a listener with a separate context so we can stop it independently.
	ctx := context.Background()
	listener := h.Env().Database.SessionFactory.NewListener(ctx, "events", km.AddEvent)
	time.Sleep(2 * time.Second)

	// Phase 1: Create a baseline event with the listener healthy.
	baselineResources, err := h.CreateResourceList(consumer.Name, 1)
	Expect(err).NotTo(HaveOccurred())
	baselineCreateTime := time.Now()

	Eventually(func() error {
		mu.Lock()
		defer mu.Unlock()
		if _, ok := processedTimes[baselineResources[0].ID]; !ok {
			return fmt.Errorf("baseline event not yet processed")
		}
		return nil
	}, 10*time.Second, 100*time.Millisecond).Should(Succeed())

	mu.Lock()
	baselineLatency := processedTimes[baselineResources[0].ID].Sub(baselineCreateTime)
	mu.Unlock()
	t.Logf("Baseline event latency (listener healthy): %v", baselineLatency)

	// Phase 2: Close the listener
	listener.Close()
	time.Sleep(500 * time.Millisecond)

	// Phase 3: Create events while the listener is down.
	// pg_notify fires but the listener goroutine has exited, so notifications
	// are not forwarded to the controller's queue.
	numChaosEvents := 5
	chaosResources, err := h.CreateResourceList(consumer.Name, numChaosEvents)
	Expect(err).NotTo(HaveOccurred())
	chaosCreateTime := time.Now()
	t.Logf("Created %d events at %v with listener down", numChaosEvents, chaosCreateTime)

	// Phase 4: Wait for syncEventsIfQueueIsEmpty to discover and process them.
	// Expected: ~5s (1s poll interval + up to 4s backoff when no events found).
	Eventually(func() error {
		mu.Lock()
		defer mu.Unlock()
		missing := 0
		for _, r := range chaosResources {
			if _, ok := processedTimes[r.ID]; !ok {
				missing++
			}
		}
		if missing > 0 {
			return fmt.Errorf("%d/%d chaos events not yet processed", missing, numChaosEvents)
		}
		return nil
	}, 30*time.Second, 500*time.Millisecond).Should(Succeed())

	// Phase 5: Report latencies.
	mu.Lock()
	var maxRecoveryLatency time.Duration
	for _, r := range chaosResources {
		latency := processedTimes[r.ID].Sub(chaosCreateTime)
		if latency > maxRecoveryLatency {
			maxRecoveryLatency = latency
		}
		t.Logf("  Chaos event %s recovery latency: %v", r.ID[:8], latency)
	}
	mu.Unlock()

	t.Logf("Max recovery latency after listener drop: %v", maxRecoveryLatency)
	t.Logf("Baseline latency with healthy listener:   %v", baselineLatency)

	if maxRecoveryLatency > 15*time.Second {
		t.Errorf("Recovery latency %v exceeded 15s threshold — syncEventsIfQueueIsEmpty may not be working", maxRecoveryLatency)
	}
}

// TestRepeatedListenerDrops induces multiple listener drops during a stream
// of event creation and measures per-event latency across all disruptions.
func TestRepeatedListenerDrops(t *testing.T) {
	h, _ := test.RegisterIntegration(t)

	controllerCtx, cancelController := context.WithCancel(context.Background())
	defer cancelController()

	consumer, err := h.CreateConsumer("cluster-" + rand.String(5))
	Expect(err).NotTo(HaveOccurred())
	h.StartWorkAgent(controllerCtx, consumer.Name)

	var mu sync.Mutex
	createTimes := map[string]time.Time{}
	processedTimes := map[string]time.Time{}

	onUpsert := func(ctx context.Context, id string) error {
		mu.Lock()
		defer mu.Unlock()
		if _, seen := processedTimes[id]; !seen {
			processedTimes[id] = time.Now()
		}
		return nil
	}

	h.Env().Database.SessionFactory.New(controllerCtx).Exec("DELETE FROM events")

	km := controllers.NewKindControllerManager(
		controllers.NewPredicatedEventFilter(func(ctx context.Context, eventID string) (bool, error) {
			return true, nil
		}),
		h.Env().Services.Events(),
	)
	km.Add(&controllers.ControllerConfig{
		Source: "Resources",
		Handlers: map[api.EventType][]controllers.ControllerHandlerFunc{
			api.CreateEventType: {onUpsert},
			api.UpdateEventType: {onUpsert},
		},
	})

	go km.Run(controllerCtx)
	time.Sleep(2 * time.Second)

	totalEvents := 0

	for round := 0; round < 3; round++ {
		// Start a fresh listener.
		listenerCtx, cancelListener := context.WithCancel(context.Background())
		h.Env().Database.SessionFactory.NewListener(listenerCtx, "events", km.AddEvent)
		time.Sleep(500 * time.Millisecond)

		// Stop the listener.
		cancelListener()
		time.Sleep(200 * time.Millisecond)

		// Create events while the listener is down.
		eventsThisRound := 3
		resources, err := h.CreateResourceList(consumer.Name, eventsThisRound)
		Expect(err).NotTo(HaveOccurred())
		now := time.Now()

		mu.Lock()
		for _, r := range resources {
			createTimes[r.ID] = now
		}
		mu.Unlock()
		totalEvents += eventsThisRound

		t.Logf("Round %d: created %d events with listener down", round, eventsThisRound)

		Eventually(func() error {
			mu.Lock()
			defer mu.Unlock()
			for _, r := range resources {
				if _, ok := processedTimes[r.ID]; !ok {
					return fmt.Errorf("round %d: event %s not yet processed", round, r.ID[:8])
				}
			}
			return nil
		}, 30*time.Second, 500*time.Millisecond).Should(Succeed())
	}

	mu.Lock()
	var totalLatency time.Duration
	var maxLatency time.Duration
	for id, created := range createTimes {
		processed, ok := processedTimes[id]
		if !ok {
			t.Errorf("Event %s was never processed", id[:8])
			continue
		}
		latency := processed.Sub(created)
		totalLatency += latency
		if latency > maxLatency {
			maxLatency = latency
		}
	}
	mu.Unlock()

	avgLatency := totalLatency / time.Duration(totalEvents)
	t.Logf("Across %d events over 3 listener drops:", totalEvents)
	t.Logf("  Average recovery latency: %v", avgLatency)
	t.Logf("  Max recovery latency:     %v", maxLatency)

	if maxLatency > 15*time.Second {
		t.Errorf("Max recovery latency %v exceeded 15s threshold", maxLatency)
	}
}
