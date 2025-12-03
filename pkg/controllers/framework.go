package controllers

import (
	"context"
	"fmt"
	"time"

	"github.com/openshift-online/maestro/pkg/api"
	"github.com/openshift-online/maestro/pkg/services"

	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"
)

/*
This controller pattern mimics upstream Kube-style controllers with Add/Update/Delete events with periodic
sync-the-world for any messages missed.

The implementation is specific to the Event table in this service and leverages features of PostgreSQL:

	1. pg_notify(channel, msg) is used for real time notification to listeners
	2. advisory locks are used for concurrency when doing background work

DAOs decorated similarly to the ResourceDAO will persist Events to the database and listeners are notified of the changed.
A worker attempting to process the Event will first obtain a fail-fast advisory lock. Of many competing workers, only
one would first successfully obtain the lock. All other workers will *not* wait to obtain the lock.

Any successful processing of an Event will remove it from the Events table permanently.

A periodic process reads from the Events table and calls pg_notify, ensuring any failed Events are re-processed. Competing
consumers for the lock will fail fast on redundant messages.

*/

type ControllerHandlerContextKey string

const EventID ControllerHandlerContextKey = "event"

// defaultEventsSyncPeriod is a default events sync period (10 hours)
// given a long period because we have a queue in the controller, it will help us to handle most expected errors, this
// events sync will help us to handle unexpected errors (e.g. sever restart), it ensures we will not miss any events
var defaultEventsSyncPeriod = 10 * time.Hour

type ControllerHandlerFunc func(ctx context.Context, id string) error

type ControllerConfig struct {
	Source   string
	Handlers map[api.EventType][]ControllerHandlerFunc
}

type KindControllerManager struct {
	controllers map[string]map[api.EventType][]ControllerHandlerFunc
	eventFilter EventFilter
	events      services.EventService
	eventsQueue workqueue.TypedRateLimitingInterface[string]
}

func NewKindControllerManager(eventFilter EventFilter, events services.EventService) *KindControllerManager {
	return &KindControllerManager{
		controllers: map[string]map[api.EventType][]ControllerHandlerFunc{},
		eventFilter: eventFilter,
		events:      events,
		eventsQueue: workqueue.NewTypedRateLimitingQueueWithConfig(
			workqueue.DefaultTypedControllerRateLimiter[string](),
			workqueue.TypedRateLimitingQueueConfig[string]{
				Name:            "event-controller",
				MetricsProvider: prometheusMetricsProvider{},
			},
		),
	}
}

func (km *KindControllerManager) Queue() workqueue.TypedRateLimitingInterface[string] {
	return km.eventsQueue
}

func (km *KindControllerManager) Add(config *ControllerConfig) {
	for ev, fn := range config.Handlers {
		km.add(config.Source, ev, fn)
	}
}

func (km *KindControllerManager) AddEvent(id string) {
	km.eventsQueue.Add(id)
}

func (km *KindControllerManager) Run(ctx context.Context) {
	logger := klog.FromContext(ctx)
	logger.Info("Starting event controller")
	defer km.eventsQueue.ShutDown()

	// start a goroutine to sync all events periodically
	// use a jitter to avoid multiple instances syncing the events at the same time
	go wait.JitterUntilWithContext(ctx, km.syncEvents, defaultEventsSyncPeriod, 0.25, true)

	// start a goroutine to handle the event from the event queue
	// the .Until will re-kick the runWorker one second after the runWorker completes
	go wait.UntilWithContext(ctx, km.runWorker, time.Second)

	// wait until we're told to stop
	<-ctx.Done()
	logger.Info("Shutting down event controller")
}

func (km *KindControllerManager) add(source string, ev api.EventType, fns []ControllerHandlerFunc) {
	if _, exists := km.controllers[source]; !exists {
		km.controllers[source] = map[api.EventType][]ControllerHandlerFunc{}
	}

	if _, exists := km.controllers[source][ev]; !exists {
		km.controllers[source][ev] = []ControllerHandlerFunc{}
	}

	km.controllers[source][ev] = append(km.controllers[source][ev], fns...)
}

func (km *KindControllerManager) handleEvent(ctx context.Context, id string) (bool, error) {
	logger := klog.FromContext(ctx).WithValues(EventID, id)
	reqContext := context.WithValue(klog.NewContext(ctx, logger), EventID, id)

	// check if the event should be processed by this instance
	shouldProcess, err := km.eventFilter.Filter(reqContext, id)
	defer km.eventFilter.DeferredAction(reqContext, id)
	if err != nil {
		return false, fmt.Errorf("error filtering event with id (%s): %s", id, err)
	}

	if !shouldProcess {
		// the event should not be processed by this instance at present
		// we put the event to the queue again until the event has been reconciled by
		// a certain instance.
		logger.Info("Event should not be processed by this instance")
		return false, nil
	}

	event, svcErr := km.events.Get(reqContext, id)
	if svcErr != nil {
		if svcErr.Is404() {
			// the event is already deleted, we can ignore it
			logger.Info("Event is not found")
			specEventReconciledTotal.WithLabelValues("unknown", string(controllerReconciledStatusSkipped)).Inc()
			return true, nil
		}
		specEventReconciledTotal.WithLabelValues("unknown", string(controllerReconciledStatusError)).Inc()
		return false, fmt.Errorf("error getting event with id (%s): %s", id, svcErr)
	}

	if event.ReconciledDate != nil {
		// the event is already reconciled, we can ignore it
		logger.Info("Event is already reconciled")
		specEventReconciledTotal.WithLabelValues(string(event.EventType), string(controllerReconciledStatusSkipped)).Inc()
		return true, nil
	}

	startTime := time.Now()
	defer func() {
		specEventReconcileDuration.WithLabelValues(string(event.EventType)).Observe(time.Since(startTime).Seconds())
	}()

	source, found := km.controllers[event.Source]
	if !found {
		logger.Info("No controllers found", "source", event.Source)
		specEventReconciledTotal.WithLabelValues(string(event.EventType), string(controllerReconciledStatusSkipped)).Inc()
		return true, nil
	}

	handlerFns, found := source[event.EventType]
	if !found {
		logger.Info("No handler functions found", "source", event.Source, "eventType", event.EventType)
		specEventReconciledTotal.WithLabelValues(string(event.EventType), string(controllerReconciledStatusSkipped)).Inc()
		return true, nil
	}

	for _, fn := range handlerFns {
		err := fn(reqContext, event.SourceID)
		if err != nil {
			specEventReconciledTotal.WithLabelValues(string(event.EventType), string(controllerReconciledStatusError)).Inc()
			return false, fmt.Errorf("error handing event %s-%s (%s): %s", event.Source, event.EventType, id, err)
		}
	}

	// all handlers successfully executed
	now := time.Now()
	event.ReconciledDate = &now
	if _, svcErr := km.events.Replace(reqContext, event); svcErr != nil {
		specEventReconciledTotal.WithLabelValues(string(event.EventType), string(controllerReconciledStatusError)).Inc()
		return false, fmt.Errorf("error updating event with id (%s): %s", id, svcErr)
	}

	specEventReconciledTotal.WithLabelValues(string(event.EventType), string(controllerReconciledStatusSuccess)).Inc()
	// the event is reconciled, we can ignore it
	return true, nil
}

func (km *KindControllerManager) runWorker(ctx context.Context) {
	// hot loop until we're told to stop. processNextEvent will automatically wait until there's work available, so
	// we don't worry about secondary waits
	for km.processNextEvent(ctx) {
	}
}

// processNextEvent deals with one key off the queue.
func (km *KindControllerManager) processNextEvent(ctx context.Context) bool {
	// pull the next event item from queue.
	// events queue blocks until it can return an item to be processed
	key, quit := km.eventsQueue.Get()
	if quit {
		// the current queue is shutdown and becomes empty, quit this process
		return false
	}
	defer km.eventsQueue.Done(key)

	logger := klog.FromContext(ctx).WithValues("key", key)

	if reconciled, err := km.handleEvent(ctx, key); !reconciled {
		if err != nil {
			logger.Error(err, "Failed to handle the event")
		}

		// the event is not reconciled, we requeue it to work on later
		// this method will add a backoff to avoid hotlooping on particular items
		km.eventsQueue.AddRateLimited(key)
		return true
	}

	// we handle the event successfully, tell the queue to stop tracking history for this event
	km.eventsQueue.Forget(key)
	return true
}

func (km *KindControllerManager) syncEvents(ctx context.Context) {
	logger := klog.FromContext(ctx)
	logger.Info("purge all reconciled events")
	// delete the reconciled events from the database firstly
	if err := km.events.DeleteAllReconciledEvents(ctx); err != nil {
		// this process is called periodically, so if the error happened, we will wait for the next cycle to handle
		// this again
		logger.Error(err, "Failed to delete reconciled events from db")
		specControllerSyncEventOperationsTotal.WithLabelValues(string(controllerSyncEventStatusError)).Inc()
		return
	}

	logger.Info("sync all unreconciled events")
	unreconciledEvents, err := km.events.FindAllUnreconciledEvents(ctx)
	if err != nil {
		logger.Error(err, "Failed to list unreconciled events from db")
		specControllerSyncEventOperationsTotal.WithLabelValues(string(controllerSyncEventStatusError)).Inc()
		return
	}

	// add the unreconciled events back to the controller queue
	for _, event := range unreconciledEvents {
		km.eventsQueue.Add(event.ID)
	}

	specControllerSyncEventOperationsTotal.WithLabelValues(string(controllerSyncEventStatusSuccess)).Inc()
}
