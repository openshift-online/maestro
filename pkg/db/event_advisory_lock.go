package db

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/openshift-online/maestro/pkg/logger"
	"gorm.io/gorm"
)

type IsConsumerSubscribed func(consumerName string) bool

// EventAdvisoryLockFactory creates advisory locks for resource events,
// ensuring the consumer relevant to the current event is subscribed to the current gRPC broker
// before lock creation. This should be used only when the gRPC broker is enabled.
type EventAdvisoryLockFactory struct {
	connection           SessionFactory
	lockStore            *AdvisoryLockStore
	isConsumerSubscribed IsConsumerSubscribed
}

// NewEventAdvisoryLockFactory returns a new EventAdvisoryLockFactory with an AdvisoryLockStore
// and a function to check if the consumer relevant to the current event is subscribed to the current gRPC broker.
func NewEventAdvisoryLockFactory(connection SessionFactory, isConsumerSubscribed IsConsumerSubscribed) *EventAdvisoryLockFactory {
	return &EventAdvisoryLockFactory{
		connection:           connection,
		lockStore:            NewAdvisoryLockStore(),
		isConsumerSubscribed: isConsumerSubscribed,
	}
}

// checkConsumerSubscribed checks if the consumer relevant to the current event is subscribed to the current gRPC broker.
func (f *EventAdvisoryLockFactory) checkConsumerSubscribed(ctx context.Context, id string) (bool, error) {
	g2 := (f.connection).New(ctx)

	event := map[string]interface{}{}
	if err := g2.Table("events").Take(&event, "id = ?", id).Error; err != nil {
		return false, fmt.Errorf("error getting event with id(%s): %v", id, err)
	}

	source, ok := event["source"].(string)
	if !ok || source != "Resources" {
		return false, fmt.Errorf("invalid event source %s", source)
	}

	sourceID, ok := event["source_id"].(string)
	if !ok || sourceID == "" {
		return false, fmt.Errorf("invalid event source id %s", sourceID)
	}

	resource := map[string]interface{}{}
	if err := g2.Table("resources").Take(&resource, "id = ?", sourceID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return false, nil
		}
		return false, fmt.Errorf("error getting resource with id(%s): %v", sourceID, err)
	}

	consumerName, ok := resource["consumer_name"].(string)
	if !ok || consumerName == "" {
		return false, fmt.Errorf("invalid resource consumer name %s", consumerName)
	}

	return f.isConsumerSubscribed(consumerName), nil
}

// NewAdvisoryLock creates a new advisory lock for the given id and lock type.
func (f *EventAdvisoryLockFactory) NewAdvisoryLock(ctx context.Context, id string, lockType LockType) (string, error) {
	log := logger.NewOCMLogger(ctx)
	if lockType != Events {
		return "", fmt.Errorf("invalid lock type %s", lockType)
	}

	subscribed, err := f.checkConsumerSubscribed(ctx, id)
	if err != nil {
		return "", fmt.Errorf("error checking consumer subscribed: %v", err)
	}

	if !subscribed {
		return "", fmt.Errorf("consumer not subscribed")
	}

	lock, err := f.newLock(ctx, id, lockType)
	if err != nil {
		return "", err
	}

	// obtain the advisory lock (blocking)
	if err := lock.lock(); err != nil {
		UpdateAdvisoryLockCountMetric(lockType, "lock error")
		errMsg := fmt.Sprintf("error obtaining the advisory lock for id %s type %s, %v", id, lockType, err)
		log.Error(errMsg)
		// the lock transaction is already started, if error happens, we return the transaction id, so that the caller
		// can end this transaction.
		return *lock.uuid, fmt.Errorf(errMsg)
	}

	log.V(10).Info(fmt.Sprintf("Locked advisory lock id=%s type=%s - owner=%s", id, lockType, *lock.uuid))
	f.lockStore.add(*lock.uuid, lock)
	return *lock.uuid, nil
}

// NewNonBlockingLock creates a new non-blocking advisory lock for the given id and lock type.
func (f *EventAdvisoryLockFactory) NewNonBlockingLock(ctx context.Context, id string, lockType LockType) (string, bool, error) {
	log := logger.NewOCMLogger(ctx)
	if lockType != Events {
		return "", false, fmt.Errorf("invalid lock type %s", lockType)
	}

	subscribed, err := f.checkConsumerSubscribed(ctx, id)
	if err != nil {
		return "", false, fmt.Errorf("error checking consumer subscribed: %v", err)
	}

	if !subscribed {
		return "", false, nil
	}

	lock, err := f.newLock(ctx, id, lockType)
	if err != nil {
		return "", false, err
	}

	// obtain the advisory lock (unblocking)
	acquired, err := lock.nonBlockingLock()
	if err != nil {
		UpdateAdvisoryLockCountMetric(lockType, "lock error")
		errMsg := fmt.Sprintf("error obtaining the non blocking advisory lock for id %s type %s, %v", id, lockType, err)
		log.Error(errMsg)
		// the lock transaction is already started, if error happens, we return the transaction id, so that the caller
		// can end this transaction.
		return *lock.uuid, false, fmt.Errorf(errMsg)
	}

	log.V(10).Info(fmt.Sprintf("Locked non blocking advisory lock id=%s type=%s - owner=%s", id, lockType, *lock.uuid))
	f.lockStore.add(*lock.uuid, lock)
	return *lock.uuid, acquired, nil
}

func (f *EventAdvisoryLockFactory) newLock(ctx context.Context, id string, lockType LockType) (*AdvisoryLock, error) {
	// lockOwnerID will be different for every service function that attempts to start a lock.
	// only the initial call in the stack must unlock.
	// Unlock() will compare UUIDs and ensure only the top level call succeeds.
	lockOwnerID := uuid.New().String()
	lock, err := newAdvisoryLock(ctx, f.connection)
	if err != nil {
		return nil, err
	}

	lock.uuid = &lockOwnerID
	lock.id = &id
	lock.lockType = &lockType

	return lock, nil
}

// Unlock searches current locks and unlocks the one matching its owner id.
func (f *EventAdvisoryLockFactory) Unlock(ctx context.Context, uuid string) {
	log := logger.NewOCMLogger(ctx)

	if uuid == "" {
		return
	}

	lock, ok := f.lockStore.get(uuid)
	if !ok {
		// the resolving UUID belongs to a service call that did *not* initiate the lock.
		// we can safely ignore this, knowing the top-most func in the call stack
		// will provide the correct UUID.
		log.V(10).Info(fmt.Sprintf("Caller not lock owner. Owner %s", uuid))
		return
	}

	lockType := *lock.lockType
	lockID := "<missing>"
	if lock.id != nil {
		lockID = *lock.id
	}

	if err := lock.unlock(); err != nil {
		UpdateAdvisoryLockCountMetric(lockType, "unlock error")
		log.Extra("lockID", lockID).Extra("owner", uuid).Error(fmt.Sprintf("Could not unlock, %v", err))
	}

	UpdateAdvisoryLockCountMetric(lockType, "OK")
	UpdateAdvisoryLockDurationMetric(lockType, "OK", lock.startTime)

	log.V(10).Info(fmt.Sprintf("Unlocked lock id=%s type=%s - owner=%s", lockID, lockType, uuid))
	f.lockStore.delete(uuid)
}
