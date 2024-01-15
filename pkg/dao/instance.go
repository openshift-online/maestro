package dao

import (
	"context"

	"gorm.io/gorm/clause"

	"github.com/openshift-online/maestro/pkg/api"
	"github.com/openshift-online/maestro/pkg/db"
)

type InstanceDao interface {
	Get(ctx context.Context, id string) (*api.Instance, error)
	Create(ctx context.Context, instance *api.Instance) (*api.Instance, error)
	Replace(ctx context.Context, instance *api.Instance) (*api.Instance, error)
	UpSert(ctx context.Context, instance *api.Instance) (*api.Instance, error)
	Delete(ctx context.Context, id string) error
	FindByIDs(ctx context.Context, ids []string) (api.InstanceList, error)
	All(ctx context.Context) (api.InstanceList, error)
}

var _ InstanceDao = &sqlInstanceDao{}

type sqlInstanceDao struct {
	sessionFactory *db.SessionFactory
}

func NewInstanceDao(sessionFactory *db.SessionFactory) InstanceDao {
	return &sqlInstanceDao{sessionFactory: sessionFactory}
}

func (d *sqlInstanceDao) Get(ctx context.Context, id string) (*api.Instance, error) {
	g2 := (*d.sessionFactory).New(ctx)
	var instance api.Instance
	if err := g2.Take(&instance, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &instance, nil
}

func (d *sqlInstanceDao) Create(ctx context.Context, instance *api.Instance) (*api.Instance, error) {
	g2 := (*d.sessionFactory).New(ctx)
	if err := g2.Omit(clause.Associations).Create(instance).Error; err != nil {
		db.MarkForRollback(ctx, err)
		return nil, err
	}
	return instance, nil
}

func (d *sqlInstanceDao) Replace(ctx context.Context, instance *api.Instance) (*api.Instance, error) {
	g2 := (*d.sessionFactory).New(ctx)
	if err := g2.Omit(clause.Associations).Save(instance).Error; err != nil {
		db.MarkForRollback(ctx, err)
		return nil, err
	}
	return instance, nil
}

func (d *sqlInstanceDao) UpSert(ctx context.Context, instance *api.Instance) (*api.Instance, error) {
	g2 := (*d.sessionFactory).New(ctx)
	if err := g2.Omit(clause.Associations).Save(instance).Error; err != nil {
		db.MarkForRollback(ctx, err)
		return nil, err
	}
	return instance, nil
}

func (d *sqlInstanceDao) Delete(ctx context.Context, id string) error {
	g2 := (*d.sessionFactory).New(ctx)
	if err := g2.Omit(clause.Associations).Delete(&api.Instance{Meta: api.Meta{ID: id}}).Error; err != nil {
		db.MarkForRollback(ctx, err)
		return err
	}
	return nil
}

func (d *sqlInstanceDao) FindByIDs(ctx context.Context, ids []string) (api.InstanceList, error) {
	g2 := (*d.sessionFactory).New(ctx)
	instances := api.InstanceList{}
	if err := g2.Where("id in (?)", ids).Find(&instances).Error; err != nil {
		return nil, err
	}
	return instances, nil
}

func (d *sqlInstanceDao) All(ctx context.Context) (api.InstanceList, error) {
	g2 := (*d.sessionFactory).New(ctx)
	instances := api.InstanceList{}
	if err := g2.Find(&instances).Error; err != nil {
		return nil, err
	}
	return instances, nil
}
