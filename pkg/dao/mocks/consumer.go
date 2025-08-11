package mocks

import (
	"context"
	"fmt"

	"gorm.io/gorm"

	"github.com/openshift-online/maestro/pkg/api"
	"github.com/openshift-online/maestro/pkg/dao"
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
	for i, c := range d.consumers {
		if c.ID == consumer.ID {
			d.consumers[i] = consumer
			return consumer, nil
		}
	}
	return nil, gorm.ErrRecordNotFound
}

func (d *consumerDaoMock) Delete(ctx context.Context, id string, unscoped bool) error {
	for i, consumer := range d.consumers {
		if consumer.ID == id {
			d.consumers = append(d.consumers[:i], d.consumers[i+1:]...)
			return nil
		}
	}
	return gorm.ErrRecordNotFound
}

func (d *consumerDaoMock) FindByIDs(ctx context.Context, ids []string) (api.ConsumerList, error) {
	var consumers api.ConsumerList
	for _, id := range ids {
		consumer, err := d.Get(ctx, id)
		if err == nil {
			consumers = append(consumers, consumer)
		}
	}
	if len(consumers) == 0 {
		return nil, fmt.Errorf("no consumers found with IDs: %v", ids)
	}
	return consumers, nil
}

func (d *consumerDaoMock) FindByNames(ctx context.Context, names []string) (api.ConsumerList, error) {
	var consumers api.ConsumerList
	for _, name := range names {
		for _, consumer := range d.consumers {
			if consumer.Name == name {
				consumers = append(consumers, consumer)
				break
			}
		}
	}
	if len(consumers) == 0 {
		return nil, fmt.Errorf("no consumers found with names: %v", names)
	}
	return consumers, nil
}

func (d *consumerDaoMock) All(ctx context.Context) (api.ConsumerList, error) {
	return d.consumers, nil
}
