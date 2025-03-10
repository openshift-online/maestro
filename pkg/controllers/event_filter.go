package controllers

import (
	"context"
	"fmt"

	"github.com/openshift-online/maestro/pkg/db"
)

// EventFilter defines an interface for filtering and deferring actions on events.
// Implementations of EventFilter should provide logic for determining whether an event
// should be processed and for handling any actions that need to be deferred.
//
//   - Filter: Decides whether the event should be processed based on its ID.
//   - DeferredAction: Allows for scheduling actions that should occur regardless of whether the event
//     was processed successfully or not, such as cleanup tasks or releasing resources.
type EventFilter interface {
	// Filter determines whether the event should be processed.
	// Returns true if the event should be handled, false and an error otherwise.
	Filter(ctx context.Context, id string) (bool, error)

	// DeferredAction schedules actions to be executed regardless of event processing success.
	DeferredAction(ctx context.Context, id string)
}

// LockBasedEventFilter implements EventFilter using a locking mechanism for event processing.
// It creates advisory locks on event IDs to ensure thread-safe access.
// - Filter acquires a lock on the event ID and returns true if the lock is successful.
// - DeferredAction releases the lock for the event ID.
type LockBasedEventFilter struct {
	lockFactory db.LockFactory
	// locks map is accessed by a single-threaded handler goroutine, no need for lock on it.
	locks map[string]string
}

func NewLockBasedEventFilter(lockFactory db.LockFactory) EventFilter {
	return &LockBasedEventFilter{
		lockFactory: lockFactory,
		locks:       make(map[string]string),
	}
}

// Filter attempts to acquire a lock on the event ID. Returns true if successful, false and error otherwise.
func (h *LockBasedEventFilter) Filter(ctx context.Context, id string) (bool, error) {
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
		log.Infof("Event %s is processed by another worker", id)
		return false, nil
	}

	return true, nil
}

// DeferredAction releases the lock for the given event ID if it was acquired.
func (h *LockBasedEventFilter) DeferredAction(ctx context.Context, id string) {
	if ownerID, exists := h.locks[id]; exists {
		h.lockFactory.Unlock(ctx, ownerID)
		delete(h.locks, id)
	}
}

// eventFilterPredicate is a function type for filtering events based on their ID.
type eventFilterPredicate func(ctx context.Context, eventID string) (bool, error)

// PredicatedEventFilter implements EventFilter using a predicate function for event filtering.
// - Filter uses the predicate to decide if the event should be processed.
// - DeferredAction is a no-op as no locking is performed.
type PredicatedEventFilter struct {
	predicate eventFilterPredicate
}

func NewPredicatedEventFilter(predicate eventFilterPredicate) EventFilter {
	return &PredicatedEventFilter{
		predicate: predicate,
	}
}

// Filter calls the predicate function to determine if the event should be processed.
func (h *PredicatedEventFilter) Filter(ctx context.Context, id string) (bool, error) {
	return h.predicate(ctx, id)
}

// DeferredAction is a no-op since no locks are involved.
func (h *PredicatedEventFilter) DeferredAction(ctx context.Context, id string) {
	// no-op
}
