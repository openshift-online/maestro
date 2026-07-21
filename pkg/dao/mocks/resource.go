package mocks

import (
	"context"
	"time"

	"gorm.io/gorm"

	"github.com/openshift-online/maestro/pkg/api"
	"github.com/openshift-online/maestro/pkg/dao"
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

func (d *resourceDaoMock) UpdateStatus(ctx context.Context, resource *api.Resource) (*api.Resource, error) {
	for i, r := range d.resources {
		if r.ID == resource.ID {
			d.resources[i].Status = resource.Status
			return d.resources[i], nil
		}
	}
	return nil, gorm.ErrRecordNotFound
}

func (d *resourceDaoMock) Delete(ctx context.Context, id string, unscoped bool) error {
	for i, resource := range d.resources {
		if resource.ID == id {
			if unscoped {
				// permanently remove the record
				d.resources = append(d.resources[:i], d.resources[i+1:]...)
				return nil
			}
			// soft delete: mark deleted_at, keeping the record retrievable via the
			// Unscoped Get the real DAO performs.
			if !resource.DeletedAt.Valid {
				resource.DeletedAt = gorm.DeletedAt{Time: time.Now(), Valid: true}
			}
			return nil
		}
	}
	return gorm.ErrRecordNotFound
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

// FindUndelivered returns resources with empty status older than threshold.
// Unlike the production implementation, this mock does not validate consumer
// existence/deletion or check for in-flight events - those are tested via
// integration tests.
func (d *resourceDaoMock) FindUndelivered(ctx context.Context, threshold time.Duration) (api.ResourceList, error) {
	var resources api.ResourceList
	cutoff := time.Now().Add(-threshold)
	for _, resource := range d.resources {
		if len(resource.Status) == 0 && resource.DeletedAt.Time.IsZero() && resource.CreatedAt.Before(cutoff) {
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
