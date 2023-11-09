package mocks

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/openshift-online/maestro/pkg/db"
)

type MockAdvisoryLockFactory struct {
	locks map[string]string
}

func NewMockAdvisoryLockFactory() *MockAdvisoryLockFactory {
	return &MockAdvisoryLockFactory{
		locks: make(map[string]string),
	}
}

func (f *MockAdvisoryLockFactory) NewAdvisoryLock(ctx context.Context, id string, lockType db.LockType) (string, error) {
	lockOwnerID := uuid.New().String()
	key := fmt.Sprintf("%s-%s", id, lockType)
	if _, ok := f.locks[key]; ok {
		return lockOwnerID, nil
	}

	f.locks[key] = lockOwnerID
	return lockOwnerID, nil
}

func (f *MockAdvisoryLockFactory) NewNonBlockingLock(ctx context.Context, id string, lockType db.LockType) (string, bool, error) {
	lockOwnerID := uuid.New().String()
	key := fmt.Sprintf("%s-%s", id, lockType)
	if _, ok := f.locks[key]; ok {
		return lockOwnerID, true, nil
	}

	f.locks[key] = lockOwnerID
	return lockOwnerID, true, nil
}

func (f *MockAdvisoryLockFactory) Unlock(ctx context.Context, uuid string) {
	for k, v := range f.locks {
		if v == uuid {
			delete(f.locks, k)
		}
	}
}
