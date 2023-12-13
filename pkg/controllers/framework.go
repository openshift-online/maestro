package controllers

import (
	"context"
	"fmt"
	"time"

	"github.com/openshift-online/maestro/pkg/api"
	"github.com/openshift-online/maestro/pkg/db"
	"github.com/openshift-online/maestro/pkg/logger"
	"github.com/openshift-online/maestro/pkg/services"
)

/*
This controller pattern mimics upstream Kube-style controllers with Add/Update/Delete events with periodic
sync-the-world for any messages missed.

The implementation is specific to the Event table in this service and leverages features of PostresDB:

	1. pg_notify(channel, msg) is used for real time notification to listeners
	2. advisory locks are used for concurrency when doing background work

DAOs decorated similarly to the ResourceDAO will persist Events to the database and listeners are notified of the changed.
A worker attemping to process the Event will first obtain a fail-fast adivosry lock. Of many competing workers, only
one would first successfully obtain the lock. All other workers will *not* wait to obtain the lock.

Any successful processing of an Event will remove it from the Events table permanently.

A periodic process reads from the Events table and calls pg_notify, ensuring any failed Events are re-processed. Competing
consumers for the lock will fail fast on redundant messages.

*/

type ControllerHandlerFunc func(ctx context.Context, id string) error

type ControllerConfig struct {
	Source   string
	Handlers map[api.EventType][]ControllerHandlerFunc
}

type KindControllerManager struct {
	controllers map[string]map[api.EventType][]ControllerHandlerFunc
	lockFactory db.LockFactory
	events      services.EventService
}

func NewKindControllerManager(lockFactory db.LockFactory, events services.EventService) *KindControllerManager {
	return &KindControllerManager{
		controllers: map[string]map[api.EventType][]ControllerHandlerFunc{},
		lockFactory: lockFactory,
		events:      events,
	}
}

func (km *KindControllerManager) Add(config *ControllerConfig) {
	for ev, fn := range config.Handlers {
		km.add(config.Source, ev, fn)
	}
}

func (km *KindControllerManager) add(source string, ev api.EventType, fns []ControllerHandlerFunc) {
	if _, exists := km.controllers[source]; !exists {
		km.controllers[source] = map[api.EventType][]ControllerHandlerFunc{}
	}

	if _, exists := km.controllers[source][ev]; !exists {
		km.controllers[source][ev] = []ControllerHandlerFunc{}
	}

	for _, fn := range fns {
		km.controllers[source][ev] = append(km.controllers[source][ev], fn)
	}
}

func (km *KindControllerManager) Handle(id string) {

	ctx := context.Background()
	logger := logger.NewOCMLogger(ctx)

	// lock the Event with a fail-fast advisory lock context.
	// this allows concurrent processing of many events by one or many controller managers.
	// allow the lock to be released by the handler goroutine and allow this function to continue.
	// subsequent events will be locked by their own distinct IDs.
	lockOwnerID, acquired, err := km.lockFactory.NewNonBlockingLock(ctx, id, db.Events)
	// Ensure that the transaction related to this lock always end.
	defer km.lockFactory.Unlock(ctx, lockOwnerID)
	if err != nil {
		logger.Error(fmt.Sprintf("Error obtaining the event lock: %v", err))
		return
	}
	if !acquired {
		logger.Infof("Event %s is processed by another worker, continue to process the next", id)
		return
	}
	threadContext := context.WithValue(ctx, "event", id)

	km.handle(threadContext, id)
}

func (km *KindControllerManager) handle(ctx context.Context, id string) {

	log := logger.NewOCMLogger(ctx)

	event, err := km.events.Get(ctx, id)

	if err != nil {
		log.Error(err.Error())
		return
	}

	source, found := km.controllers[event.Source]
	if !found {
		log.Infof("No controllers found for '%s'\n", event.Source)
		return
	}

	handlerFns, found := source[event.EventType]
	if !found {
		log.Infof("No handler functions found for '%s-%s'\n", event.Source, event.EventType)
		return
	}

	for _, fn := range handlerFns {
		err := fn(ctx, event.SourceID)
		if err != nil {
			errStr := fmt.Sprintf("error handing event %s, %s, %s: %s", event.Source, event.EventType, id, err)
			log.Error(errStr)
			return
		}
	}

	// all handlers successfully executed
	now := time.Now()
	event.ReconciledDate = &now
	_, err = km.events.Replace(ctx, event)
	if err != nil {
		log.Error(err.Error())
	}
}
