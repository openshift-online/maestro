package controllers

import (
	"context"
	"fmt"
	"time"

	"github.com/openshift-online/maestro/pkg/api"
	"github.com/openshift-online/maestro/pkg/dao"
	"github.com/openshift-online/maestro/pkg/services"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/util/workqueue"
)

const StatusEventID ControllerHandlerContextKey = "status_event"

type StatusHandlerFunc func(ctx context.Context, eventID, sourceID string) error

type StatusController struct {
	controllers      map[api.StatusEventType][]StatusHandlerFunc
	statusEvents     services.StatusEventService
	instanceDao      dao.InstanceDao
	eventInstanceDao dao.EventInstanceDao
	eventsQueue      workqueue.RateLimitingInterface
}

func NewStatusController(statusEvents services.StatusEventService,
	instanceDao dao.InstanceDao,
	eventInstanceDao dao.EventInstanceDao) *StatusController {
	return &StatusController{
		controllers:      map[api.StatusEventType][]StatusHandlerFunc{},
		statusEvents:     statusEvents,
		instanceDao:      instanceDao,
		eventInstanceDao: eventInstanceDao,
		eventsQueue:      workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "status-event-controller"),
	}
}

// AddStatusEvent adds a status event to the queue to be processed.
func (sc *StatusController) AddStatusEvent(id string) {
	sc.eventsQueue.Add(id)
}

func (sc *StatusController) Run(stopCh <-chan struct{}) {
	log.Infof("Starting status event controller")
	defer sc.eventsQueue.ShutDown()

	// use a jitter to avoid multiple instances syncing the events at the same time
	go wait.JitterUntil(sc.syncStatusEvents, defaultEventsSyncPeriod, 0.25, true, stopCh)

	// start a goroutine to handle the status event from the event queue
	// the .Until will re-kick the runWorker one second after the runWorker completes
	go wait.Until(sc.runWorker, time.Second, stopCh)

	// wait until we're told to stop
	<-stopCh
	log.Infof("Shutting down status event controller")
}

func (sc *StatusController) runWorker() {
	// hot loop until we're told to stop. processNextEvent will automatically wait until there's work available, so
	// we don't worry about secondary waits
	for sc.processNextEvent() {
	}
}

// processNextEvent deals with one key off the queue.
func (sc *StatusController) processNextEvent() bool {
	// pull the next status event item from queue.
	// events queue blocks until it can return an item to be processed
	key, quit := sc.eventsQueue.Get()
	if quit {
		// the current queue is shutdown and becomes empty, quit this process
		return false
	}
	defer sc.eventsQueue.Done(key)

	if err := sc.handleStatusEvent(key.(string)); err != nil {
		log.Errorf("Failed to handle the event %v, %v ", key, err)

		// we failed to handle the status event, we should requeue the item to work on later
		// this method will add a backoff to avoid hotlooping on particular items
		sc.eventsQueue.AddRateLimited(key)
		return true
	}

	// we handle the status event successfully, tell the queue to stop tracking history for this status event
	sc.eventsQueue.Forget(key)
	return true
}

// syncStatusEvents handles the status event with the given ID.
// It reads the status event from the database and is called on each replica
// without locking, ensuring the status event is broadcast to all subscribers.
func (sc *StatusController) handleStatusEvent(id string) error {
	ctx := context.Background()
	reqContext := context.WithValue(ctx, StatusEventID, id)
	statusEvent, svcErr := sc.statusEvents.Get(reqContext, id)
	if svcErr != nil {
		if svcErr.Is404() {
			// the status event is already deleted, we can ignore it
			return nil
		}
		return fmt.Errorf("error getting status event with id(%s): %s", id, svcErr)
	}

	if statusEvent.ReconciledDate != nil {
		return nil
	}

	handlerFns, found := sc.controllers[statusEvent.StatusEventType]
	if !found {
		log.Infof("No handler functions found for status event '%s'\n", statusEvent.StatusEventType)
		return nil
	}

	for _, fn := range handlerFns {
		err := fn(reqContext, id, statusEvent.ResourceID)
		if err != nil {
			return fmt.Errorf("error handling status event %s, %s, %s: %s", statusEvent.StatusEventType, id, statusEvent.ResourceID, err)
		}
	}

	return nil
}

func (sc *StatusController) Add(handlers map[api.StatusEventType][]StatusHandlerFunc) {
	for ev, fn := range handlers {
		sc.add(ev, fn)
	}
}

func (sc *StatusController) add(ev api.StatusEventType, fns []StatusHandlerFunc) {
	if _, exists := sc.controllers[ev]; !exists {
		sc.controllers[ev] = []StatusHandlerFunc{}
	}

	sc.controllers[ev] = append(sc.controllers[ev], fns...)
}

func (sc *StatusController) syncStatusEvents() {
	ctx := context.Background()

	readyInstanceIDs, err := sc.instanceDao.FindReadyIDs(ctx)
	if err != nil {
		log.Errorf("Failed to find ready instances from db, %v", err)
		return
	}
	log.Infof("purge status events on the ready instances: %s", readyInstanceIDs)

	// find the status events that already were dispatched to all ready instances
	statusEventIDs, err := sc.eventInstanceDao.GetEventsAssociatedWithInstances(ctx, readyInstanceIDs)
	if err != nil {
		log.Errorf("Failed to find handled status events from db, %v", err)
		return
	}

	// batch delete the handled status events
	batches := batchStatusEventIDs(statusEventIDs, 500)
	for _, batch := range batches {
		if err := sc.statusEvents.DeleteAllEvents(ctx, batch); err != nil {
			log.Errorf("Failed to delete handled status events from db, %v", err)
			return
		}
	}
}

func batchStatusEventIDs(statusEventIDs []string, batchSize int) [][]string {
	batches := [][]string{}
	for i := 0; i < len(statusEventIDs); i += batchSize {
		end := i + batchSize
		if end > len(statusEventIDs) {
			end = len(statusEventIDs)
		}
		batches = append(batches, statusEventIDs[i:end])
	}
	return batches
}
