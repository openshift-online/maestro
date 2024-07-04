package controllers

import (
	"context"
	"fmt"
	"time"

	"github.com/openshift-online/maestro/pkg/api"
	"github.com/openshift-online/maestro/pkg/services"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/util/workqueue"
)

const SpecEventID ControllerHandlerContextKey = "spec_event"

type SpecHandlerFunc func(ctx context.Context, eventID, resourceID string) error

type SpecController struct {
	controllers map[api.EventType][]SpecHandlerFunc
	events      services.EventService
	eventsQueue workqueue.RateLimitingInterface
}

func NewSpecController(events services.EventService) *SpecController {
	return &SpecController{
		controllers: map[api.EventType][]SpecHandlerFunc{},
		events:      events,
		eventsQueue: workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "spec-event-controller"),
	}
}

// AddSpecEvent adds a spec event to the queue to be processed.
func (sc *SpecController) AddSpecEvent(id string) {
	sc.eventsQueue.Add(id)
}

func (sc *SpecController) Run(stopCh <-chan struct{}) {
	logger.Infof("Starting spec event controller")
	defer sc.eventsQueue.ShutDown()

	// TODO: start a goroutine to sync all spec events periodically
	// use a jitter to avoid multiple instances syncing the events at the same time
	// go wait.JitterUntil(sc.syncSpecEvents, defaultEventsSyncPeriod, 0.25, true, stopCh)

	// start a goroutine to handle the spec event from the event queue
	// the .Until will re-kick the runWorker one second after the runWorker completes
	go wait.Until(sc.runWorker, time.Second, stopCh)

	// wait until we're told to stop
	<-stopCh
	logger.Infof("Shutting down spec event controller")
}

func (sc *SpecController) runWorker() {
	// hot loop until we're told to stop. processNextEvent will automatically wait until there's work available, so
	// we don't worry about secondary waits
	for sc.processNextEvent() {
	}
}

// processNextEvent deals with one key off the queue.
func (sc *SpecController) processNextEvent() bool {
	// pull the next spec event item from queue.
	// events queue blocks until it can return an item to be processed
	key, quit := sc.eventsQueue.Get()
	if quit {
		// the current queue is shutdown and becomes empty, quit this process
		return false
	}
	defer sc.eventsQueue.Done(key)

	if err := sc.handleSpecEvent(key.(string)); err != nil {
		logger.Error(fmt.Sprintf("Failed to handle the event %v, %v ", key, err))

		// we failed to handle the spec event, we should requeue the item to work on later
		// this method will add a backoff to avoid hotlooping on particular items
		sc.eventsQueue.AddRateLimited(key)
		return true
	}

	// we handle the status event successfully, tell the queue to stop tracking history for this spec event
	sc.eventsQueue.Forget(key)
	return true
}

// handleSpecEvent handles the spec event with the given ID.
// It reads the spec from the database and is called on each replica
// without locking, ensuring the spec event is broadcast to all subscribers.
func (sc *SpecController) handleSpecEvent(id string) error {
	ctx := context.Background()
	reqContext := context.WithValue(ctx, SpecEventID, id)
	specEvent, svcErr := sc.events.Get(reqContext, id)
	if svcErr != nil {
		if svcErr.Is404() {
			// the specevent is already deleted, we can ignore it
			return nil
		}
		return fmt.Errorf("error getting spec event with id(%s): %s", id, svcErr)
	}

	// if specEvent.ReconciledDate != nil {
	// 	return nil
	// }

	// only handle spec events from the "Resources" source
	if specEvent.Source != "Resources" {
		return nil
	}

	handlerFns, found := sc.controllers[specEvent.EventType]
	if !found {
		logger.Infof("No handler functions found for spec event '%s'\n", specEvent.EventType)
		return nil
	}

	for _, fn := range handlerFns {
		err := fn(reqContext, id, specEvent.SourceID)
		if err != nil {
			return fmt.Errorf("error handling spec event %s, %s: %s", specEvent.EventType, id, err)
		}
	}

	return nil
}

func (sc *SpecController) Add(handlers map[api.EventType][]SpecHandlerFunc) {
	for ev, fn := range handlers {
		sc.add(ev, fn)
	}
}

func (sc *SpecController) add(ev api.EventType, fns []SpecHandlerFunc) {
	if _, exists := sc.controllers[ev]; !exists {
		sc.controllers[ev] = []SpecHandlerFunc{}
	}

	sc.controllers[ev] = append(sc.controllers[ev], fns...)
}
