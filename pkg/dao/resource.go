package dao

import (
	"context"

	"gorm.io/gorm/clause"

	"github.com/openshift-online/maestro/pkg/api"
	"github.com/openshift-online/maestro/pkg/db"
)

type ResourceDao interface {
	Get(ctx context.Context, id string) (*api.Resource, error)
	Create(ctx context.Context, dinosaur *api.Resource) (*api.Resource, error)
	Replace(ctx context.Context, dinosaur *api.Resource) (*api.Resource, error)
	Delete(ctx context.Context, id string) error
	FindByIDs(ctx context.Context, ids []string) (api.ResourceList, error)
	FindByConsumerID(ctx context.Context, consumerID string) (api.ResourceList, error)
	All(ctx context.Context) (api.ResourceList, error)
}

var _ ResourceDao = &sqlResourceDao{}

type sqlResourceDao struct {
	sessionFactory *db.SessionFactory
}

func NewResourceDao(sessionFactory *db.SessionFactory) ResourceDao {
	return &sqlResourceDao{sessionFactory: sessionFactory}
}

func (d *sqlResourceDao) Get(ctx context.Context, id string) (*api.Resource, error) {
	g2 := (*d.sessionFactory).New(ctx)
	var dinosaur api.Resource
	if err := g2.Take(&dinosaur, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &dinosaur, nil
}

func (d *sqlResourceDao) Create(ctx context.Context, dinosaur *api.Resource) (*api.Resource, error) {
	g2 := (*d.sessionFactory).New(ctx)
	if err := g2.Omit(clause.Associations).Create(dinosaur).Error; err != nil {
		db.MarkForRollback(ctx, err)
		return nil, err
	}
	return dinosaur, nil
}

func (d *sqlResourceDao) Replace(ctx context.Context, dinosaur *api.Resource) (*api.Resource, error) {
	g2 := (*d.sessionFactory).New(ctx)
	if err := g2.Omit(clause.Associations).Save(dinosaur).Error; err != nil {
		db.MarkForRollback(ctx, err)
		return nil, err
	}
	return dinosaur, nil
}

func (d *sqlResourceDao) Delete(ctx context.Context, id string) error {
	g2 := (*d.sessionFactory).New(ctx)
	if err := g2.Omit(clause.Associations).Delete(&api.Resource{Meta: api.Meta{ID: id}}).Error; err != nil {
		db.MarkForRollback(ctx, err)
		return err
	}
	return nil
}

func (d *sqlResourceDao) FindByIDs(ctx context.Context, ids []string) (api.ResourceList, error) {
	g2 := (*d.sessionFactory).New(ctx)
	dinosaurs := api.ResourceList{}
	if err := g2.Where("id in (?)", ids).Find(&dinosaurs).Error; err != nil {
		return nil, err
	}
	return dinosaurs, nil
}

func (d *sqlResourceDao) FindByConsumerID(ctx context.Context, consumerID string) (api.ResourceList, error) {
	g2 := (*d.sessionFactory).New(ctx)
	dinosaurs := api.ResourceList{}
	if err := g2.Where("consumerID = ?", consumerID).Find(&dinosaurs).Error; err != nil {
		return nil, err
	}
	return dinosaurs, nil
}

func (d *sqlResourceDao) All(ctx context.Context) (api.ResourceList, error) {
	g2 := (*d.sessionFactory).New(ctx)
	dinosaurs := api.ResourceList{}
	if err := g2.Find(&dinosaurs).Error; err != nil {
		return nil, err
	}
	return dinosaurs, nil
}
