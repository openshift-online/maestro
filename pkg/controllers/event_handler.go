package controllers

import (
	"context"
	"fmt"
	"time"

	"github.com/openshift-online/maestro/pkg/api"
	"github.com/openshift-online/maestro/pkg/dao"
	"github.com/openshift-online/maestro/pkg/db"
	"github.com/openshift-online/maestro/pkg/services"
	"k8s.io/klog/v2"
)

// EventHandler defines the actions to handle an event at various stages of its lifecycle.
type EventHandler interface {
	// ShouldHandleEvent determines whether the event should be processed.
	// Returns true if the event should be handled, false and an error otherwise.
	ShouldHandleEvent(ctx context.Context, id string) (bool, error)

	// DeferredAction schedules any deferred actions that need to be executed
	// after the event is processed successfully or unsuccessfully.
	DeferredAction(ctx context.Context, id string)

	// PostProcess is called after the event is processed to perform any cleanup
	// or additional actions required for the event.
	PostProcess(ctx context.Context, event *api.Event) error
}

// LockBasedEventHandler is an implementation of EventHandler that uses a locking mechanism to control event processing.
// It leverages a lock factory to create advisory locks for each event ID, ensuring non-blocking, thread-safe access.
// - ShouldHandleEvent acquires the lock for the event ID and returns true if the lock is successful.
// - DeferredAction releases the lock for the event ID.
// - PostProcess updates the event with a reconciled date after processing.
type LockBasedEventHandler struct {
	lockFactory db.LockFactory
	locks       map[string]string
	events      services.EventService
}

func NewLockBasedEventHandler(lockFactory db.LockFactory, events services.EventService) EventHandler {
	return &LockBasedEventHandler{
		lockFactory: lockFactory,
		locks:       make(map[string]string),
		events:      events,
	}
}

func (h *LockBasedEventHandler) ShouldHandleEvent(ctx context.Context, id string) (bool, error) {
	// lock the Event with a fail-fast advisory lock context.
	// this allows concurrent processing of many events by one or many controller managers.
	// allow the lock to be released by the handler goroutine and allow this function to continue.
	// subsequent events will be locked by their own distinct IDs.
	lockOwnerID, acquired, err := h.lockFactory.NewNonBlockingLock(ctx, id, db.Events)
	// store the lock owner ID for deferred action
	h.locks[id] = lockOwnerID
	if err != nil {
		return false, fmt.Errorf("error obtaining the event lock: %v", err)
	}

	if !acquired {
		logger.V(4).Infof("Event %s is processed by another worker", id)
		return false, nil
	}

	return true, nil
}

func (h *LockBasedEventHandler) DeferredAction(ctx context.Context, id string) {
	if ownerID, exists := h.locks[id]; exists {
		h.lockFactory.Unlock(ctx, ownerID)
		delete(h.locks, id)
	}
}

func (h *LockBasedEventHandler) PostProcess(ctx context.Context, event *api.Event) error {
	// update the event with the reconciled date
	if event != nil {
		now := time.Now()
		event.ReconciledDate = &now
		if _, svcErr := h.events.Replace(ctx, event); svcErr != nil {
			return fmt.Errorf("error updating event with id(%s): %s", event.ID, svcErr)
		}
	}

	return nil
}

// eventHandlerPredicate is a function type for filtering events based on their ID.
type eventHandlerPredicate func(ctx context.Context, eventID string) (bool, error)

// PredicatedEventHandler is an implementation of EventHandler that filters events using a predicate function.
//   - ShouldHandleEvent uses the predicate to determine if the event should be processed by ID.
//   - DeferredAction is a no-op as no locking is performed.
//   - PostProcess updates the event with the reconciled date and checks if it's processed by all instances.
//     If all instances have processed the event, it marks the event as reconciled.
type PredicatedEventHandler struct {
	predicate        eventHandlerPredicate
	events           services.EventService
	eventInstanceDao dao.EventInstanceDao
	instanceDao      dao.InstanceDao
}

func NewPredicatedEventHandler(predicate eventHandlerPredicate, events services.EventService, eventInstanceDao dao.EventInstanceDao, instanceDao dao.InstanceDao) EventHandler {
	return &PredicatedEventHandler{
		predicate:        predicate,
		events:           events,
		eventInstanceDao: eventInstanceDao,
		instanceDao:      instanceDao,
	}
}

func (h *PredicatedEventHandler) ShouldHandleEvent(ctx context.Context, id string) (bool, error) {
	return h.predicate(ctx, id)
}

func (h *PredicatedEventHandler) DeferredAction(ctx context.Context, id string) {
	// no-op
}

func (h *PredicatedEventHandler) PostProcess(ctx context.Context, event *api.Event) error {
	// check the event and alive instances
	// if the event is handled by all alive instances, mark the event as reconciled
	activeInstances, err := h.instanceDao.FindReadyIDs(ctx)
	if err != nil {
		return fmt.Errorf("error finding ready instances: %v", err)
	}

	processedInstances, err := h.eventInstanceDao.GetInstancesBySpecEventID(ctx, event.ID)
	if err != nil {
		return fmt.Errorf("error finding processed instances for event %s: %v", event.ID, err)
	}

	// should never happen. If the event is not processed by any instance, return an error
	if len(processedInstances) == 0 {
		klog.V(10).Infof("Event %s is not processed by any instance", event.ID)
		return fmt.Errorf("event %s is not processed by any instance", event.ID)
	}

	// check if all instances have processed the event
	// 1. In normal case, the activeInstances == eventInstances, mark the event as reconciled
	// 2. If maestro server instance is up, but has't been marked as ready, then activeInstances < eventInstances,
	// it's ok to mark the event as reconciled, as the instance is not ready to sever the request, no connected agents.
	// 3. If maestro server instance is down, but has been marked as unready, it may still have connected agents, but
	// the instance has stopped to handle the event, so activeInstances > eventInstances, the event should be equeued.
	if !isSubSet(activeInstances, processedInstances) {
		klog.V(10).Infof("Event %s is not processed by all active instances %v, handled by %v", event.ID, activeInstances, processedInstances)
		return fmt.Errorf("event %s is not processed by all active instances", event.ID)
	}

	// update the event with the reconciled date
	now := time.Now()
	event.ReconciledDate = &now
	if _, svcErr := h.events.Replace(ctx, event); svcErr != nil {
		return fmt.Errorf("error updating event with id(%s): %s", event.ID, svcErr)
	}

	return nil
}

// isSubSet checks if slice a is a subset of slice b
func isSubSet(a, b []string) bool {
	for _, v := range a {
		found := false
		for _, vv := range b {
			if v == vv {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	return true
}
