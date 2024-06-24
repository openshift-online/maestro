package controllers

import (
	"context"
	"fmt"
	"time"

	"github.com/openshift-online/maestro/pkg/api"
	"github.com/openshift-online/maestro/pkg/db"
	maestrologger "github.com/openshift-online/maestro/pkg/logger"
	"github.com/openshift-online/maestro/pkg/services"

	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/util/workqueue"
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

var logger = maestrologger.NewOCMLogger(context.Background())

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
	lockFactory db.LockFactory
	events      services.EventService
	eventsQueue workqueue.RateLimitingInterface
}

func NewKindControllerManager(lockFactory db.LockFactory, events services.EventService) *KindControllerManager {
	return &KindControllerManager{
		controllers: map[string]map[api.EventType][]ControllerHandlerFunc{},
		lockFactory: lockFactory,
		events:      events,
		eventsQueue: workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "event-controller"),
	}
}

func (km *KindControllerManager) Add(config *ControllerConfig) {
	for ev, fn := range config.Handlers {
		km.add(config.Source, ev, fn)
	}
}

func (km *KindControllerManager) AddEvent(id string) {
	km.eventsQueue.Add(id)
}

func (km *KindControllerManager) Run(stopCh <-chan struct{}) {
	logger.Infof("Starting event controller")
	defer km.eventsQueue.ShutDown()

	// start a goroutine to sync all events periodically
	// use a jitter to avoid multiple instances syncing the events at the same time
	go wait.JitterUntil(km.syncEvents, defaultEventsSyncPeriod, 0.25, true, stopCh)

	// start a goroutine to handle the event from the event queue
	// the .Until will re-kick the runWorker one second after the runWorker completes
	go wait.Until(km.runWorker, time.Second, stopCh)

	// wait until we're told to stop
	<-stopCh
	logger.Infof("Shutting down event controller")
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

func (km *KindControllerManager) handleEvent(id string) error {
	ctx := context.Background()
	// lock the Event with a fail-fast advisory lock context.
	// this allows concurrent processing of many events by one or many controller managers.
	// allow the lock to be released by the handler goroutine and allow this function to continue.
	// subsequent events will be locked by their own distinct IDs.
	lockOwnerID, acquired, err := km.lockFactory.NewNonBlockingLock(ctx, id, db.Events)
	// Ensure that the transaction related to this lock always end.
	defer km.lockFactory.Unlock(ctx, lockOwnerID)
	if err != nil {
		return fmt.Errorf("error obtaining the event lock: %v", err)
	}

	if !acquired {
		logger.Infof("Event %s is processed by another worker, continue to process the next", id)
		return nil
	}

	reqContext := context.WithValue(ctx, EventID, id)

	event, svcErr := km.events.Get(reqContext, id)
	if svcErr != nil {
		if svcErr.Is404() {
			// the event is already deleted, we can ignore it
			return nil
		}
		return fmt.Errorf("error getting event with id(%s): %s", id, svcErr)
	}

	if event.ReconciledDate != nil {
		return nil
	}

	source, found := km.controllers[event.Source]
	if !found {
		logger.Infof("No controllers found for '%s'\n", event.Source)
		return nil
	}

	handlerFns, found := source[event.EventType]
	if !found {
		logger.Infof("No handler functions found for '%s-%s'\n", event.Source, event.EventType)
		return nil
	}

	for _, fn := range handlerFns {
		err := fn(reqContext, event.SourceID)
		if err != nil {
			return fmt.Errorf("error handing event %s, %s, %s: %s", event.Source, event.EventType, id, err)
		}
	}

	// all handlers successfully executed
	now := time.Now()
	event.ReconciledDate = &now
	_, svcErr = km.events.Replace(reqContext, event)
	if svcErr != nil {
		return fmt.Errorf("error updating event with id(%s): %s", id, svcErr)
	}
	return nil
}

func (km *KindControllerManager) runWorker() {
	// hot loop until we're told to stop. processNextEvent will automatically wait until there's work available, so
	// we don't worry about secondary waits
	for km.processNextEvent() {
	}
}

// processNextEvent deals with one key off the queue.
func (km *KindControllerManager) processNextEvent() bool {
	// pull the next event item from queue.
	// events queue blocks until it can return an item to be processed
	key, quit := km.eventsQueue.Get()
	if quit {
		// the current queue is shutdown and becomes empty, quit this process
		return false
	}
	defer km.eventsQueue.Done(key)

	if err := km.handleEvent(key.(string)); err != nil {
		logger.Error(fmt.Sprintf("Failed to handle the event %v, %v ", key, err))

		// we failed to handle the event, we should requeue the item to work on later
		// this method will add a backoff to avoid hotlooping on particular items
		km.eventsQueue.AddRateLimited(key)
		return true
	}

	// we handle the event successfully, tell the queue to stop tracking history for this event
	km.eventsQueue.Forget(key)
	return true
}

func (km *KindControllerManager) syncEvents() {
	// delete the reconciled events from the database firstly
	if err := km.events.DeleteAllReconciledEvents(context.Background()); err != nil {
		// this process is called periodically, so if the error happened, we will wait for the next cycle to handle
		// this again
		logger.Error(fmt.Sprintf("Failed to delete reconciled events from db, %v", err))
		return
	}

	unreconciledEvents, err := km.events.FindAllUnreconciledEvents(context.Background())
	if err != nil {
		logger.Error(fmt.Sprintf("Failed to list unreconciled events from db, %v", err))
		return
	}

	// add the unreconciled events back to the controller queue
	for _, event := range unreconciledEvents {
		km.eventsQueue.Add(event.ID)
	}
}
