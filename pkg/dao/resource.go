package dao

import (
	"context"

	"gorm.io/gorm/clause"

	"github.com/openshift-online/maestro/pkg/api"
	"github.com/openshift-online/maestro/pkg/db"
)

type ResourceDao interface {
	Get(ctx context.Context, id string) (*api.Resource, error)
	Create(ctx context.Context, resource *api.Resource) (*api.Resource, error)
	Update(ctx context.Context, resource *api.Resource) (*api.Resource, error)
	Delete(ctx context.Context, id string, unscoped bool) error
	FindByIDs(ctx context.Context, ids []string) (api.ResourceList, error)
	FindBySource(ctx context.Context, source string) (api.ResourceList, error)
	FindByConsumerName(ctx context.Context, consumerName string) (api.ResourceList, error)
	All(ctx context.Context) (api.ResourceList, error)
	FirstByConsumerName(ctx context.Context, name string, unscoped bool) (api.Resource, error)
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
	var resource api.Resource
	if err := g2.Unscoped().Take(&resource, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &resource, nil
}

func (d *sqlResourceDao) Create(ctx context.Context, resource *api.Resource) (*api.Resource, error) {
	g2 := (*d.sessionFactory).New(ctx)
	if err := g2.Omit(clause.Associations).Create(resource).Error; err != nil {
		db.MarkForRollback(ctx, err)
		return nil, err
	}
	return resource, nil
}

func (d *sqlResourceDao) Update(ctx context.Context, resource *api.Resource) (*api.Resource, error) {
	g2 := (*d.sessionFactory).New(ctx)
	if err := g2.Omit(clause.Associations).Updates(resource).Error; err != nil {
		db.MarkForRollback(ctx, err)
		return nil, err
	}
	return resource, nil
}

func (d *sqlResourceDao) Delete(ctx context.Context, id string, unscoped bool) error {
	g2 := (*d.sessionFactory).New(ctx)
	if unscoped {
		// Unscoped is used to permanently delete the record
		g2 = g2.Unscoped()
	}
	if err := g2.Omit(clause.Associations).Delete(&api.Resource{Meta: api.Meta{ID: id}}).Error; err != nil {
		db.MarkForRollback(ctx, err)
		return err
	}
	return nil
}

func (d *sqlResourceDao) FindByIDs(ctx context.Context, ids []string) (api.ResourceList, error) {
	g2 := (*d.sessionFactory).New(ctx)
	resources := api.ResourceList{}
	if err := g2.Unscoped().Where("id in (?)", ids).Find(&resources).Error; err != nil {
		return nil, err
	}
	return resources, nil
}

func (d *sqlResourceDao) FindBySource(ctx context.Context, source string) (api.ResourceList, error) {
	g2 := (*d.sessionFactory).New(ctx)
	resources := api.ResourceList{}
	if err := g2.Unscoped().Where("source = ?", source).Find(&resources).Error; err != nil {
		return nil, err
	}
	return resources, nil
}

func (d *sqlResourceDao) FindByConsumerName(ctx context.Context, consumerName string) (api.ResourceList, error) {
	g2 := (*d.sessionFactory).New(ctx)
	resources := api.ResourceList{}
	if err := g2.Unscoped().Where("consumer_name = ?", consumerName).Find(&resources).Error; err != nil {
		return nil, err
	}
	return resources, nil
}

func (d *sqlResourceDao) All(ctx context.Context) (api.ResourceList, error) {
	g2 := (*d.sessionFactory).New(ctx)
	resources := api.ResourceList{}
	if err := g2.Unscoped().Find(&resources).Error; err != nil {
		return nil, err
	}
	return resources, nil
}

// FirstByConsumerName will take the first item of the resources on the consumer. it can be used to determine whether the resource exists for the consumer.
func (d *sqlResourceDao) FirstByConsumerName(ctx context.Context, consumerName string, unscoped bool) (api.Resource, error) {
	g2 := (*d.sessionFactory).New(ctx)
	if unscoped {
		// Unscoped is used to find the deleting resources
		g2 = g2.Unscoped()
	}
	resource := api.Resource{}
	err := g2.Where("consumer_name = ?", consumerName).First(&resource).Error
	return resource, err
}
