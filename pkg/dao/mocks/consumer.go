package mocks

import (
	"context"

	"gorm.io/gorm"

	"github.com/openshift-online/maestro/pkg/api"
	"github.com/openshift-online/maestro/pkg/dao"
	"github.com/openshift-online/maestro/pkg/errors"
)

var _ dao.ConsumerDao = &consumerDaoMock{}

type consumerDaoMock struct {
	consumers api.ConsumerList
}

func NewConsumerDao() *consumerDaoMock {
	return &consumerDaoMock{}
}

func (d *consumerDaoMock) Get(ctx context.Context, id string) (*api.Consumer, error) {
	for _, consumer := range d.consumers {
		if consumer.ID == id {
			return consumer, nil
		}
	}
	return nil, gorm.ErrRecordNotFound
}

func (d *consumerDaoMock) Create(ctx context.Context, consumer *api.Consumer) (*api.Consumer, error) {
	d.consumers = append(d.consumers, consumer)
	return consumer, nil
}

func (d *consumerDaoMock) Replace(ctx context.Context, consumer *api.Consumer) (*api.Consumer, error) {
	return nil, errors.NotImplemented("Consumer").AsError()
}

func (d *consumerDaoMock) Delete(ctx context.Context, id string) error {
	return errors.NotImplemented("Consumer").AsError()
}

func (d *consumerDaoMock) FindByIDs(ctx context.Context, ids []string) (api.ConsumerList, error) {
	return nil, errors.NotImplemented("Consumer").AsError()
}

func (d *consumerDaoMock) All(ctx context.Context) (api.ConsumerList, error) {
	return d.consumers, nil
}
