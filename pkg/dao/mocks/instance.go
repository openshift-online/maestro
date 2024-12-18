package mocks

import (
	"context"
	"fmt"
	"sync"
	"time"

	"gorm.io/gorm"

	"github.com/openshift-online/maestro/pkg/api"
	"github.com/openshift-online/maestro/pkg/dao"
	"github.com/openshift-online/maestro/pkg/errors"
)

var _ dao.InstanceDao = &instanceDaoMock{}

type instanceDaoMock struct {
	mux       sync.RWMutex
	instances api.ServerInstanceList
}

func NewInstanceDao() *instanceDaoMock {
	return &instanceDaoMock{}
}

func (d *instanceDaoMock) Get(ctx context.Context, id string) (*api.ServerInstance, error) {
	d.mux.RLock()
	defer d.mux.RUnlock()
	for _, instance := range d.instances {
		if instance.ID == id {
			return instance, nil
		}
	}
	return nil, gorm.ErrRecordNotFound
}

func (d *instanceDaoMock) Create(ctx context.Context, instance *api.ServerInstance) (*api.ServerInstance, error) {
	d.mux.Lock()
	defer d.mux.Unlock()
	d.instances = append(d.instances, instance)
	return instance, nil
}

func (d *instanceDaoMock) Replace(ctx context.Context, instance *api.ServerInstance) (*api.ServerInstance, error) {
	d.mux.Lock()
	defer d.mux.Unlock()
	for i, inst := range d.instances {
		if inst.ID == instance.ID {
			d.instances[i] = instance
			return instance, nil
		}
	}
	return nil, gorm.ErrRecordNotFound
}

func (d *instanceDaoMock) UpSert(ctx context.Context, instance *api.ServerInstance) (*api.ServerInstance, error) {
	d.mux.Lock()
	defer d.mux.Unlock()
	for i, inst := range d.instances {
		if inst.ID == instance.ID {
			d.instances[i] = instance
			return instance, nil
		}
	}
	d.instances = append(d.instances, instance)
	return instance, nil
}

func (d *instanceDaoMock) MarkReadyByIDs(ctx context.Context, ids []string) error {
	d.mux.Lock()
	defer d.mux.Unlock()
	for _, instance := range d.instances {
		if contains(ids, instance.ID) {
			instance.Ready = true
		}
	}
	return nil
}

func (d *instanceDaoMock) MarkUnreadyByIDs(ctx context.Context, ids []string) error {
	d.mux.Lock()
	defer d.mux.Unlock()
	for _, instance := range d.instances {
		if contains(ids, instance.ID) {
			instance.Ready = false
		}
	}
	return nil
}

func (d *instanceDaoMock) Delete(ctx context.Context, ID string) error {
	d.mux.Lock()
	defer d.mux.Unlock()
	for i, instance := range d.instances {
		if instance.ID == ID {
			d.instances = append(d.instances[:i], d.instances[i+1:]...)
			return nil
		}
	}
	return fmt.Errorf("instance with ID %s not found", ID)
}

func (d *instanceDaoMock) DeleteByIDs(ctx context.Context, ids []string) error {
	d.mux.Lock()
	defer d.mux.Unlock()
	i := 0
	for _, instance := range d.instances {
		if !contains(ids, instance.ID) {
			d.instances[i] = instance
			i++
		}
	}

	for n := len(d.instances); i < n; i++ {
		d.instances[i] = nil
	}

	d.instances = d.instances[:i]
	return nil
}

func (d *instanceDaoMock) FindByIDs(ctx context.Context, ids []string) (api.ServerInstanceList, error) {
	return nil, errors.NotImplemented("Instance").AsError()
}

func (d *instanceDaoMock) FindReadyIDs(ctx context.Context) ([]string, error) {
	d.mux.RLock()
	defer d.mux.RUnlock()
	ids := make([]string, 0, len(d.instances))
	for _, instance := range d.instances {
		if instance.Ready {
			ids = append(ids, instance.ID)
		}
	}
	return ids, nil
}

func (d *instanceDaoMock) FindByUpdatedTime(ctx context.Context, updatedTime time.Time) (api.ServerInstanceList, error) {
	d.mux.RLock()
	defer d.mux.RUnlock()
	instances := api.ServerInstanceList{}
	for _, instance := range d.instances {
		if !instance.UpdatedAt.After(updatedTime) {
			instances = append(instances, instance)
		}
	}
	return instances, nil
}

func (d *instanceDaoMock) FindReadyIDs(ctx context.Context) ([]string, error) {
	d.mux.RLock()
	defer d.mux.RUnlock()

	ids := []string{}
	for _, instance := range d.instances {
		if instance.Ready {
			ids = append(ids, instance.ID)
		}
	}
	return ids, nil
}

func (d *instanceDaoMock) All(ctx context.Context) (api.ServerInstanceList, error) {
	d.mux.RLock()
	defer d.mux.RUnlock()
	return d.instances, nil
}

func contains(ids []string, id string) bool {
	for _, i := range ids {
		if i == id {
			return true
		}
	}
	return false
}
