package services

import (
	"context"
	"reflect"

	"github.com/openshift-online/maestro/pkg/dao"
	"github.com/openshift-online/maestro/pkg/db"
	logger "github.com/openshift-online/maestro/pkg/logger"

	"github.com/openshift-online/maestro/pkg/api"
	"github.com/openshift-online/maestro/pkg/errors"
)

type ResourceService interface {
	Get(ctx context.Context, id string) (*api.Resource, *errors.ServiceError)
	Create(ctx context.Context, resource *api.Resource) (*api.Resource, *errors.ServiceError)
	Replace(ctx context.Context, resource *api.Resource) (*api.Resource, *errors.ServiceError)
	Delete(ctx context.Context, id string) *errors.ServiceError
	All(ctx context.Context) (api.ResourceList, *errors.ServiceError)

	FindByConsumerIDs(ctx context.Context, consumerID string) (api.ResourceList, *errors.ServiceError)
	FindByIDs(ctx context.Context, ids []string) (api.ResourceList, *errors.ServiceError)

	// idempotent functions for the control plane, but can also be called synchronously by any actor
	OnUpsert(ctx context.Context, id string) error
	OnDelete(ctx context.Context, id string) error
}

func NewResourceService(lockFactory db.LockFactory, resourceDao dao.ResourceDao, events EventService) ResourceService {
	return &sqlResourceService{
		lockFactory: lockFactory,
		resourceDao: resourceDao,
		events:      events,
	}
}

var _ ResourceService = &sqlResourceService{}

type sqlResourceService struct {
	lockFactory db.LockFactory
	resourceDao dao.ResourceDao
	events      EventService
}

func (s *sqlResourceService) OnUpsert(ctx context.Context, id string) error {
	logger := logger.NewOCMLogger(ctx)

	resource, err := s.resourceDao.Get(ctx, id)
	if err != nil {
		return err
	}

	logger.Infof("Do idempotent somethings with this resource: %s", resource.ID)

	return nil
}

func (s *sqlResourceService) OnDelete(ctx context.Context, id string) error {
	logger := logger.NewOCMLogger(ctx)
	logger.Infof("This dino didn't make it to the asteroid: %s", id)
	return nil
}

func (s *sqlResourceService) Get(ctx context.Context, id string) (*api.Resource, *errors.ServiceError) {
	resource, err := s.resourceDao.Get(ctx, id)
	if err != nil {
		return nil, handleGetError("Resource", "id", id, err)
	}
	return resource, nil
}

func (s *sqlResourceService) Create(ctx context.Context, resource *api.Resource) (*api.Resource, *errors.ServiceError) {
	resource, err := s.resourceDao.Create(ctx, resource)
	if err != nil {
		return nil, handleCreateError("Resource", err)
	}

	_, eErr := s.events.Create(ctx, &api.Event{
		Source:    "Resources",
		SourceID:  resource.ID,
		EventType: api.CreateEventType,
	})
	if eErr != nil {
		return nil, handleCreateError("Resource", err)
	}

	return resource, nil
}

func (s *sqlResourceService) Replace(ctx context.Context, resource *api.Resource) (*api.Resource, *errors.ServiceError) {
	// Updates the resource manifest only when its manifest changes.
	// If there are multiple requests at the same time, it will cause the race conditions among these
	// requests (read–modify–write), the advisory lock is used here to prevent the race conditions.
	lockOwnerID, err := s.lockFactory.NewAdvisoryLock(ctx, resource.ID, db.Resources)
	if err != nil {
		return nil, errors.DatabaseAdvisoryLock(err)
	}
	defer s.lockFactory.Unlock(ctx, lockOwnerID)

	found, err := s.resourceDao.Get(ctx, resource.ID)
	if err != nil {
		return nil, handleGetError("Resource", "id", resource.ID, err)
	}

	// Make sure the requested resource version is consistent with its database version.
	if found.Version != resource.Version {
		return nil, errors.Conflict("the resource version is not the latest, the latest version: %d", found.Version)
	}

	// New manifest is not changed, the update action is not needed.
	if reflect.DeepEqual(resource.Manifest, found.Manifest) {
		return found, nil
	}

	// Increase the current resource version and update its manifest.
	found.Version = found.Version + 1
	found.Manifest = resource.Manifest

	updated, err := s.resourceDao.Replace(ctx, found)
	if err != nil {
		return nil, handleUpdateError("Resource", err)
	}

	if _, err := s.events.Create(ctx, &api.Event{
		Source:    "Resources",
		SourceID:  updated.ID,
		EventType: api.UpdateEventType,
	}); err != nil {
		return nil, handleUpdateError("Resource", err)
	}
	return updated, nil
}

func (s *sqlResourceService) Delete(ctx context.Context, id string) *errors.ServiceError {
	if err := s.resourceDao.Delete(ctx, id); err != nil {
		return handleDeleteError("Resource", errors.GeneralError("Unable to delete resource: %s", err))
	}

	_, err := s.events.Create(ctx, &api.Event{
		Source:    "Resources",
		SourceID:  id,
		EventType: api.DeleteEventType,
	})
	if err != nil {
		return handleDeleteError("Resource", err)
	}

	return nil
}

func (s *sqlResourceService) FindByIDs(ctx context.Context, ids []string) (api.ResourceList, *errors.ServiceError) {
	resources, err := s.resourceDao.FindByIDs(ctx, ids)
	if err != nil {
		return nil, errors.GeneralError("Unable to get all resources: %s", err)
	}
	return resources, nil
}

func (s *sqlResourceService) FindByConsumerIDs(ctx context.Context, consumerID string) (api.ResourceList, *errors.ServiceError) {
	resources, err := s.resourceDao.FindByConsumerID(ctx, consumerID)
	if err != nil {
		return nil, handleGetError("Resource", "consumerID", consumerID, err)
	}
	return resources, nil
}

func (s *sqlResourceService) All(ctx context.Context) (api.ResourceList, *errors.ServiceError) {
	resources, err := s.resourceDao.All(ctx)
	if err != nil {
		return nil, errors.GeneralError("Unable to get all resources: %s", err)
	}
	return resources, nil
}
