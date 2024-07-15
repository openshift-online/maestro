package mocks

import (
	"context"

	"github.com/openshift-online/maestro/pkg/dao"

	"gorm.io/gorm"

	"github.com/openshift-online/maestro/pkg/api"
	"github.com/openshift-online/maestro/pkg/errors"
)

var _ dao.ResourceDao = &resourceDaoMock{}

type resourceDaoMock struct {
	resources api.ResourceList
}

func NewResourceDao() *resourceDaoMock {
	return &resourceDaoMock{}
}

func (d *resourceDaoMock) Get(ctx context.Context, id string) (*api.Resource, error) {
	for _, resource := range d.resources {
		if resource.ID == id {
			return resource, nil
		}
	}
	return nil, gorm.ErrRecordNotFound
}

func (d *resourceDaoMock) Create(ctx context.Context, resource *api.Resource) (*api.Resource, error) {
	d.resources = append(d.resources, resource)
	return resource, nil
}

func (d *resourceDaoMock) Update(ctx context.Context, resource *api.Resource) (*api.Resource, error) {
	return nil, errors.NotImplemented("Resource").AsError()
}

func (d *resourceDaoMock) Delete(ctx context.Context, id string, unscoped bool) error {
	return errors.NotImplemented("Resource").AsError()
}

func (d *resourceDaoMock) FindByIDs(ctx context.Context, ids []string) (api.ResourceList, error) {
	return nil, errors.NotImplemented("Resource").AsError()
}

func (d *resourceDaoMock) FindByConsumerName(ctx context.Context, consumerID string) (api.ResourceList, error) {
	var resources api.ResourceList
	for _, resource := range d.resources {
		if resource.ConsumerName == consumerID {
			resources = append(resources, resource)
		}
	}
	return resources, nil
}

func (d *resourceDaoMock) FindBySource(ctx context.Context, source string) (api.ResourceList, error) {
	var resources api.ResourceList
	for _, resource := range d.resources {
		if resource.Source == source {
			resources = append(resources, resource)
		}
	}
	return resources, nil
}

func (d *resourceDaoMock) All(ctx context.Context) (api.ResourceList, error) {
	return d.resources, nil
}

func (d *resourceDaoMock) FirstByConsumerName(ctx context.Context, consumerName string, unscoped bool) (api.Resource, error) {
	return *d.resources[0], errors.NotImplemented("Resource").AsError()
}
