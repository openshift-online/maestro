package dao

import (
	"context"

	"gorm.io/gorm/clause"

	"github.com/openshift-online/maestro/pkg/api"
	"github.com/openshift-online/maestro/pkg/db"
)

type ConsumerDao interface {
	Get(ctx context.Context, id string) (*api.Consumer, error)
	Create(ctx context.Context, consumer *api.Consumer) (*api.Consumer, error)
	Replace(ctx context.Context, consumer *api.Consumer) (*api.Consumer, error)
	Delete(ctx context.Context, id string) error
	GetByName(ctx context.Context, name string) (*api.Consumer, error)
	FindByIDs(ctx context.Context, ids []string) (api.ConsumerList, error)
	All(ctx context.Context) (api.ConsumerList, error)
}

var _ ConsumerDao = &sqlConsumerDao{}

type sqlConsumerDao struct {
	sessionFactory *db.SessionFactory
}

func NewConsumerDao(sessionFactory *db.SessionFactory) ConsumerDao {
	return &sqlConsumerDao{sessionFactory: sessionFactory}
}

func (d *sqlConsumerDao) Get(ctx context.Context, id string) (*api.Consumer, error) {
	g2 := (*d.sessionFactory).New(ctx)
	var consumer api.Consumer
	if err := g2.Take(&consumer, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &consumer, nil
}

func (d *sqlConsumerDao) Create(ctx context.Context, consumer *api.Consumer) (*api.Consumer, error) {
	g2 := (*d.sessionFactory).New(ctx)
	if err := g2.Omit(clause.Associations).Create(consumer).Error; err != nil {
		db.MarkForRollback(ctx, err)
		return nil, err
	}
	return consumer, nil
}

func (d *sqlConsumerDao) Replace(ctx context.Context, consumer *api.Consumer) (*api.Consumer, error) {
	g2 := (*d.sessionFactory).New(ctx)
	if err := g2.Omit(clause.Associations).Save(consumer).Error; err != nil {
		db.MarkForRollback(ctx, err)
		return nil, err
	}
	return consumer, nil
}

func (d *sqlConsumerDao) Delete(ctx context.Context, id string) error {
	g2 := (*d.sessionFactory).New(ctx)
	if err := g2.Omit(clause.Associations).Delete(&api.Consumer{Meta: api.Meta{ID: id}}).Error; err != nil {
		db.MarkForRollback(ctx, err)
		return err
	}
	return nil
}

func (d *sqlConsumerDao) GetByName(ctx context.Context, name string) (*api.Consumer, error) {
	g2 := (*d.sessionFactory).New(ctx)
	var consumer api.Consumer
	if err := g2.Take(&consumer, "name = ?", name).Error; err != nil {
		return nil, err
	}
	return &consumer, nil
}

func (d *sqlConsumerDao) FindByIDs(ctx context.Context, ids []string) (api.ConsumerList, error) {
	g2 := (*d.sessionFactory).New(ctx)
	consumers := api.ConsumerList{}
	if err := g2.Where("id in (?)", ids).Find(&consumers).Error; err != nil {
		return nil, err
	}
	return consumers, nil
}

func (d *sqlConsumerDao) All(ctx context.Context) (api.ConsumerList, error) {
	g2 := (*d.sessionFactory).New(ctx)
	consumers := api.ConsumerList{}
	if err := g2.Find(&consumers).Error; err != nil {
		return nil, err
	}
	return consumers, nil
}
