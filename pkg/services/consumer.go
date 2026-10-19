package services

import (
	"context"
	stderrors "errors"
	"math"
	"time"

	"gorm.io/gorm"
	"k8s.io/klog/v2"

	"github.com/openshift-online/maestro/pkg/api"
	"github.com/openshift-online/maestro/pkg/dao"
	"github.com/openshift-online/maestro/pkg/errors"
)

type ConsumerService interface {
	Get(ctx context.Context, id string) (*api.Consumer, *errors.ServiceError)
	Create(ctx context.Context, consumer *api.Consumer) (*api.Consumer, *errors.ServiceError)
	Replace(ctx context.Context, consumer *api.Consumer) (*api.Consumer, *errors.ServiceError)
	Delete(ctx context.Context, id string) *errors.ServiceError
	All(ctx context.Context) (api.ConsumerList, *errors.ServiceError)

	FindByIDs(ctx context.Context, ids []string) (api.ConsumerList, *errors.ServiceError)
	FindByNames(ctx context.Context, names []string) (api.ConsumerList, *errors.ServiceError)
}

// NewConsumerService preserves ordinary consumer deletion with foreign-key protection.
func NewConsumerService(consumerDao dao.ConsumerDao) ConsumerService {
	return &sqlConsumerService{
		consumerDao: consumerDao,
	}
}

// NewConsumerServiceWithCascade enables explicit finalization for decommissioned consumers.
func NewConsumerServiceWithCascade(consumerDao dao.ConsumerDao, cascade dao.ConsumerCascadeDao,
	thresholdSeconds int) ConsumerService {
	var threshold time.Duration
	if thresholdSeconds > 0 && int64(thresholdSeconds) <= math.MaxInt64/int64(time.Second) {
		threshold = time.Duration(thresholdSeconds) * time.Second
	}
	return &sqlConsumerService{
		consumerDao:    consumerDao,
		cascade:        cascade,
		staleThreshold: threshold,
	}
}

var _ ConsumerService = &sqlConsumerService{}

type sqlConsumerService struct {
	consumerDao    dao.ConsumerDao
	cascade        dao.ConsumerCascadeDao
	staleThreshold time.Duration
}

func (s *sqlConsumerService) Get(ctx context.Context, id string) (*api.Consumer, *errors.ServiceError) {
	consumer, err := s.consumerDao.Get(ctx, id)
	if err != nil {
		return nil, handleGetError("Consumer", "id", id, err)
	}
	return consumer, nil
}

func (s *sqlConsumerService) Create(ctx context.Context, consumer *api.Consumer) (*api.Consumer, *errors.ServiceError) {
	if consumer.Name != "" {
		if err := ValidateConsumer(consumer); err != nil {
			return nil, handleCreateError("Consumer", err)
		}
	}

	consumer, err := s.consumerDao.Create(ctx, consumer)
	if err != nil {
		return nil, handleCreateError("Consumer", err)
	}
	return consumer, nil
}

func (s *sqlConsumerService) Replace(ctx context.Context, consumer *api.Consumer) (*api.Consumer, *errors.ServiceError) {
	consumer, err := s.consumerDao.Replace(ctx, consumer)
	if err != nil {
		return nil, handleUpdateError("Consumer", err)
	}
	return consumer, nil
}

// Delete hard-deletes the consumer, preserving FK protection unless the opt-in
// cascade is configured to finalize only its eligible soft-deleted resources.
func (s *sqlConsumerService) Delete(ctx context.Context, id string) *errors.ServiceError {
	if s.cascade != nil {
		return s.deleteWithCascade(ctx, id)
	}
	if err := s.consumerDao.Delete(ctx, id, true); err != nil {
		return handleDeleteError("Consumer", err)
	}
	return nil
}

// deleteWithCascade coordinates terminal acknowledgements and records only committed finalizations.
// The consumer advisory lock is taken inside the cascade transaction on the connection that
// performs the protected writes, so this path never pins a second pooled connection.
func (s *sqlConsumerService) deleteWithCascade(ctx context.Context, id string) *errors.ServiceError {
	if s.staleThreshold <= 0 {
		return errors.Forbidden("Consumer delete cascade requires a positive stale threshold")
	}
	consumer, err := s.consumerDao.Get(ctx, id)
	if stderrors.Is(err, gorm.ErrRecordNotFound) {
		return nil
	}
	if err != nil {
		return handleGetError("Consumer", "id", id, err)
	}
	resources, err := s.cascade.Delete(ctx, id, consumer.Name, s.staleThreshold, buildConsumerCascadeEvent)
	if stderrors.Is(err, dao.ErrConsumerCascadeBlocked) {
		return errors.Forbidden("Unable to delete consumer: all resources must be soft-deleted longer than the stale threshold")
	}
	if err != nil {
		return handleDeleteError("Consumer", err)
	}
	consumerCascadedResourcesTotal.Add(float64(len(resources)))
	for _, resource := range resources {
		klog.FromContext(ctx).Info("Finalized soft-deleted resource during explicit consumer deletion",
			"resourceID", resource.ID, "consumerID", id)
	}
	return nil
}

func (s *sqlConsumerService) FindByIDs(ctx context.Context, ids []string) (api.ConsumerList, *errors.ServiceError) {
	consumers, err := s.consumerDao.FindByIDs(ctx, ids)
	if err != nil {
		return nil, errors.GeneralError("Unable to get all consumers: %s", err)
	}
	return consumers, nil
}

func (s *sqlConsumerService) FindByNames(ctx context.Context, names []string) (api.ConsumerList, *errors.ServiceError) {
	consumers, err := s.consumerDao.FindByNames(ctx, names)
	if err != nil {
		return nil, errors.GeneralError("Unable to find consumers by names: %s", err)
	}
	return consumers, nil
}

func (s *sqlConsumerService) All(ctx context.Context) (api.ConsumerList, *errors.ServiceError) {
	consumers, err := s.consumerDao.All(ctx)
	if err != nil {
		return nil, errors.GeneralError("Unable to get all consumers: %s", err)
	}
	return consumers, nil
}
