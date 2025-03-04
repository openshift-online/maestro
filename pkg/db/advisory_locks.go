package db

import (
	"context"
	"errors"
	"fmt"
	"hash/fnv"
	"sync"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type (
	LockType string
)

const (
	Migrations     LockType = "migrations"
	Resources      LockType = "resources"
	ResourceStatus LockType = "resource_status"
	Events         LockType = "events"
	Instances      LockType = "instances"
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

// AdvisoryLockStore is a thread-safe map that stores AdvisoryLocks.
// Map access is unsafe only when updates are occurring.
// As long as all goroutines are only reading—looking up elements in the map,
// including iterating through it using a for range loop and not changing
// the map by assigning to elements or doing deletions,
// it is safe for them to access the map concurrently without synchronization.
type AdvisoryLockStore struct {
	advisoryLockMap map[string]*AdvisoryLock
	mutex           *sync.Mutex
}

// NewAdvisoryLockStore returns a new AdvisoryLockStore.
func NewAdvisoryLockStore() *AdvisoryLockStore {
	return &AdvisoryLockStore{
		advisoryLockMap: make(map[string]*AdvisoryLock),
		mutex:           &sync.Mutex{},
	}
}

func (s *AdvisoryLockStore) add(uuid string, lock *AdvisoryLock) {
	s.mutex.Lock()
	s.advisoryLockMap[uuid] = lock
	defer s.mutex.Unlock()
}

func (s *AdvisoryLockStore) delete(uuid string) {
	s.mutex.Lock()
	delete(s.advisoryLockMap, uuid)
	defer s.mutex.Unlock()
}

func (s *AdvisoryLockStore) get(uuid string) (*AdvisoryLock, bool) {
	s.mutex.Lock()
	lock, ok := s.advisoryLockMap[uuid]
	defer s.mutex.Unlock()
	return lock, ok
}

type AdvisoryLockFactory struct {
	connection SessionFactory
	lockStore  *AdvisoryLockStore
}

// NewAdvisoryLockFactory returns a new factory with AdvisoryLock stored in it.
func NewAdvisoryLockFactory(connection SessionFactory) *AdvisoryLockFactory {
	return &AdvisoryLockFactory{
		connection: connection,
		lockStore:  NewAdvisoryLockStore(),
	}
}

func (f *AdvisoryLockFactory) NewAdvisoryLock(ctx context.Context, id string, lockType LockType) (string, error) {
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

	log.Debugf("Locked advisory lock id=%s type=%s - owner=%s", id, lockType, *lock.uuid)
	f.lockStore.add(*lock.uuid, lock)
	return *lock.uuid, nil
}

func (f *AdvisoryLockFactory) NewNonBlockingLock(ctx context.Context, id string, lockType LockType) (string, bool, error) {
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

	log.Debugf("Locked non blocking advisory lock id=%s type=%s - owner=%s", id, lockType, *lock.uuid)
	f.lockStore.add(*lock.uuid, lock)
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
	if uuid == "" {
		return
	}

	lock, ok := f.lockStore.get(uuid)
	if !ok {
		// the resolving UUID belongs to a service call that did *not* initiate the lock.
		// we can safely ignore this, knowing the top-most func in the call stack
		// will provide the correct UUID.
		log.Debugf("Caller not lock owner. Owner %s", uuid)
		return
	}

	lockType := *lock.lockType
	lockID := "<missing>"
	if lock.id != nil {
		lockID = *lock.id
	}

	if err := lock.unlock(); err != nil {
		UpdateAdvisoryLockCountMetric(lockType, "unlock error")
		log.With("lockID", lockID).With("lockType", lockType).With("owner", uuid).Errorf("error unlocking advisory lock: %v", err)
	}

	UpdateAdvisoryLockCountMetric(lockType, "OK")
	UpdateAdvisoryLockDurationMetric(lockType, "OK", lock.startTime)

	log.Debugf("Unlocked lock id=%s type=%s - owner=%s", lockID, lockType, uuid)
	f.lockStore.delete(uuid)
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
