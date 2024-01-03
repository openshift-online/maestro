package mocks

import (
	"context"
	"fmt"
	"sync"

	"gorm.io/gorm"

	"github.com/openshift-online/maestro/pkg/api"
	"github.com/openshift-online/maestro/pkg/dao"
	"github.com/openshift-online/maestro/pkg/errors"
)

var _ dao.InstanceDao = &instanceDaoMock{}

type instanceDaoMock struct {
	mux       sync.RWMutex
	instances api.InstanceList
}

func NewInstanceDao() *instanceDaoMock {
	return &instanceDaoMock{}
}

func (d *instanceDaoMock) Get(ctx context.Context, id string) (*api.Instance, error) {
	d.mux.RLock()
	defer d.mux.RUnlock()
	for _, instance := range d.instances {
		if instance.ID == id {
			return instance, nil
		}
	}
	return nil, gorm.ErrRecordNotFound
}

func (d *instanceDaoMock) Create(ctx context.Context, instance *api.Instance) (*api.Instance, error) {
	d.mux.Lock()
	defer d.mux.Unlock()
	d.instances = append(d.instances, instance)
	return instance, nil
}

func (d *instanceDaoMock) Replace(ctx context.Context, instance *api.Instance) (*api.Instance, error) {
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

func (d *instanceDaoMock) UpSert(ctx context.Context, instance *api.Instance) (*api.Instance, error) {
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

func (d *instanceDaoMock) FindByIDs(ctx context.Context, ids []string) (api.InstanceList, error) {
	return nil, errors.NotImplemented("Instance").AsError()
}

func (d *instanceDaoMock) All(ctx context.Context) (api.InstanceList, error) {
	d.mux.RLock()
	defer d.mux.RUnlock()
	return d.instances, nil
}
