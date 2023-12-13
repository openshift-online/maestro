package db

import (
	"context"
	"errors"
	"fmt"
	"hash/fnv"
	"time"

	"github.com/google/uuid"
	"github.com/openshift-online/maestro/pkg/logger"
	"gorm.io/gorm"
)

type (
	advisoryLockMap map[string]*AdvisoryLock
	LockType        string
)

const (
	Migrations LockType = "migrations"
	Resources  LockType = "Resources"
	Events     LockType = "events"
)

// LockFactory provides the blocking/unblocking locks based on PostgreSQL advisory lock.
type LockFactory interface {
	// NewAdvisoryLock constructs a new AdvisoryLock that is a blocking PostgreSQL advisory lock
	// defined by (id, lockType) and returns a UUID as this AdvisoryLock owner id.
	NewAdvisoryLock(ctx context.Context, id string, lockType LockType) (string, error)
	// NewNonBlockingLock constructs a new nonblocking AdvisoryLock defined by (id, lockType),
	// returns a UUID and a boolean on whether the lock is acquired.
	NewNonBlockingLock(ctx context.Context, id string, lockType LockType) (string, bool, error)
	// Unlock unlocks one AdvisoryLock by its owner id.
	Unlock(ctx context.Context, uuid string)
}

type AdvisoryLockFactory struct {
	connection SessionFactory
	locks      advisoryLockMap
}

// NewAdvisoryLockFactory returns a new factory with AdvisoryLock stored in it.
func NewAdvisoryLockFactory(connection SessionFactory) *AdvisoryLockFactory {
	return &AdvisoryLockFactory{
		connection: connection,
		locks:      make(advisoryLockMap),
	}
}

func (f *AdvisoryLockFactory) NewAdvisoryLock(ctx context.Context, id string, lockType LockType) (string, error) {
	log := logger.NewOCMLogger(ctx)

	lock, err := f.newLock(ctx, id, lockType)
	if err != nil {
		return "", err
	}

	// obtain the advisory lock (blocking)
	if err := lock.lock(); err != nil {
		UpdateAdvisoryLockCountMetric(lockType, "lock error")
		errMsg := fmt.Sprintf("error obtaining the advisory lock for id %s with (%d, %d), %v",
			id, hash(*lock.id), hash(string(*lock.lockType)), err)
		log.Error(errMsg)
		// the lock transaction is already started, if error happens, we return the transaction id, so that the caller
		// can end this transaction.
		return *lock.uuid, fmt.Errorf(errMsg)
	}

	f.locks[fmt.Sprintf("%s-%s", id, lockType)] = lock
	return *lock.uuid, nil
}

func (f *AdvisoryLockFactory) NewNonBlockingLock(ctx context.Context, id string, lockType LockType) (string, bool, error) {
	log := logger.NewOCMLogger(ctx)

	lock, err := f.newLock(ctx, id, lockType)
	if err != nil {
		return "", false, err
	}

	// obtain the advisory lock (unblocking)
	acquired, err := lock.nonBlockingLock()
	if err != nil {
		UpdateAdvisoryLockCountMetric(lockType, "lock error")
		errMsg := fmt.Sprintf("error obtaining the non blocking advisory lock for id %s with (%d, %d), %v",
			id, hash(*lock.id), hash(string(*lock.lockType)), err)
		log.Error(errMsg)
		// the lock transaction is already started, if error happens, we return the transaction id, so that the caller
		// can end this transaction.
		return *lock.uuid, false, fmt.Errorf(errMsg)
	}

	f.locks[fmt.Sprintf("%s-%s", id, lockType)] = lock
	return *lock.uuid, acquired, nil
}

func (f *AdvisoryLockFactory) newLock(ctx context.Context, id string, lockType LockType) (*AdvisoryLock, error) {
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
func (f *AdvisoryLockFactory) Unlock(ctx context.Context, uuid string) {
	log := logger.NewOCMLogger(ctx)

	for k, lock := range f.locks {
		if lock.uuid == nil {
			log.Error("lockOwnerID could not be found in AdvisoryLock")
			continue
		}

		if *lock.uuid != uuid {
			continue
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

		log.Info(fmt.Sprintf("Unlocked lock id=%s - owner=%s", lockID, uuid))

		delete(f.locks, k)
		return
	}

	// the resolving UUID belongs to a service call that did *not* initiate the lock.
	// we can safely ignore this, knowing the top-most func in the call stack
	// will provide the correct UUID.
	// This will happen frequently as many pkg/service functions participate in locks.
	log.Info(fmt.Sprintf("Caller not lock owner. Owner %s", uuid))
}

// AdvisoryLock represents a postgres advisory lock
//
//	begin                                       # start a Tx
//	select pg_advisory_xact_lock(id, lockType)  # obtain the lock (blocking)
//	end                                         # end the Tx and release the lock
//
// UUID is a way to own the lock. Only the very first
// service call that owns the lock will have the correct UUID. This is necessary
// to allow functions to call other service functions as part of the same lock (id, lockType).
type AdvisoryLock struct {
	g2        *gorm.DB
	txid      int64
	uuid      *string
	id        *string
	lockType  *LockType
	startTime time.Time
}

// newAdvisoryLock constructs a new AdvisoryLock object.
func newAdvisoryLock(ctx context.Context, connection SessionFactory) (*AdvisoryLock, error) {
	// it requires a new DB session to start the advisory lock.
	g2 := connection.New(ctx)

	// start a Tx to ensure gorm will obtain/release the lock using a same connection.
	tx := g2.Begin()
	if tx.Error != nil {
		return nil, tx.Error
	}

	// current transaction ID set by postgres.  these are *not* distinct across time
	// and do get reset after postgres performs "vacuuming" to reclaim used IDs.
	var txid struct{ ID int64 }
	tx.Raw("select txid_current() as id").Scan(&txid)

	return &AdvisoryLock{
		txid:      txid.ID,
		g2:        tx,
		startTime: time.Now(),
	}, nil
}

// lock calls select pg_advisory_xact_lock(id, lockType) to obtain the lock defined by (id, lockType).
// it is blocked if some other thread currently is holding the same lock (id, lockType).
// if blocked, it can be unblocked or timed out when overloaded.
func (l *AdvisoryLock) lock() error {
	if l.g2 == nil {
		return errors.New("AdvisoryLock: transaction is missing")
	}
	if l.id == nil {
		return errors.New("AdvisoryLock: id is missing")
	}
	if l.lockType == nil {
		return errors.New("AdvisoryLock: lockType is missing")
	}

	idAsInt := hash(*l.id)
	typeAsInt := hash(string(*l.lockType))
	err := l.g2.Exec("select pg_advisory_xact_lock(?, ?)", idAsInt, typeAsInt).Error
	if err != nil {
		return err
	}
	return nil
}

func (l *AdvisoryLock) nonBlockingLock() (bool, error) {
	if l.g2 == nil {
		return false, errors.New("AdvisoryLock: transaction is missing")
	}
	if l.id == nil {
		return false, errors.New("AdvisoryLock: id is missing")
	}
	if l.lockType == nil {
		return false, errors.New("AdvisoryLock: lockType is missing")
	}

	idAsInt := hash(*l.id)
	typeAsInt := hash(string(*l.lockType))
	var acquired bool
	var result string
	err := l.g2.Raw("select pg_try_advisory_xact_lock(?, ?)", idAsInt, typeAsInt).Scan(&result).Error
	if err != nil {
		return false, err
	}
	if result == "true" {
		acquired = true
	}

	return acquired, nil
}

func (l *AdvisoryLock) unlock() error {
	if l.g2 == nil {
		return errors.New("AdvisoryLock: transaction is missing")
	}

	// it ends the Tx and implicitly releases the lock.
	err := l.g2.Commit().Error
	l.g2 = nil
	l.uuid = nil
	l.id = nil
	l.lockType = nil
	return err
}

// hash string to int32 (postgres integer)
// https://pkg.go.dev/math#pkg-constants
// https://www.postgresql.org/docs/12/datatype-numeric.html
func hash(s string) int32 {
	h := fnv.New32a()
	h.Write([]byte(s))
	// Sum32() returns uint32. needs conversion.
	return int32(h.Sum32())
}
