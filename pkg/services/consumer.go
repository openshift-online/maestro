package services

import (
	"context"

	"github.com/openshift-online/maestro/pkg/dao"
	"github.com/openshift-online/maestro/pkg/db"

	"github.com/openshift-online/maestro/pkg/api"
	"github.com/openshift-online/maestro/pkg/errors"
)

type ConsumerService interface {
	Get(ctx context.Context, id string) (*api.Consumer, *errors.ServiceError)
	Create(ctx context.Context, consumer *api.Consumer) (*api.Consumer, *errors.ServiceError)
	Replace(ctx context.Context, consumer *api.Consumer) (*api.Consumer, *errors.ServiceError)
	Delete(ctx context.Context, id string) *errors.ServiceError
	All(ctx context.Context) (api.ConsumerList, *errors.ServiceError)

	FindByIDs(ctx context.Context, ids []string) (api.ConsumerList, *errors.ServiceError)
}

func NewConsumerService(lockFactory db.LockFactory, consumerDao dao.ConsumerDao, events EventService) ConsumerService {
	return &sqlConsumerService{
		consumerDao: consumerDao,
	}
}

var _ ConsumerService = &sqlConsumerService{}

type sqlConsumerService struct {
	consumerDao dao.ConsumerDao
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

func (s *sqlConsumerService) Delete(ctx context.Context, id string) *errors.ServiceError {
	if err := s.consumerDao.Delete(ctx, id); err != nil {
		return handleDeleteError("Consumer", errors.GeneralError("Unable to delete consumer: %s", err))
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

func (s *sqlConsumerService) All(ctx context.Context) (api.ConsumerList, *errors.ServiceError) {
	consumers, err := s.consumerDao.All(ctx)
	if err != nil {
		return nil, errors.GeneralError("Unable to get all consumers: %s", err)
	}
	return consumers, nil
}
