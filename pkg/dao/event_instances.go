package dao

import (
	"context"

	"gorm.io/gorm/clause"

	"github.com/openshift-online/maestro/pkg/api"
	"github.com/openshift-online/maestro/pkg/db"
)

type EventInstanceDao interface {
	Get(ctx context.Context, eventID, instanceID string) (*api.EventInstance, error)
	Create(ctx context.Context, eventInstance *api.EventInstance) (*api.EventInstance, error)
	CreateList(ctx context.Context, eventInstanceList api.EventInstanceList) error
	MarkAsDone(ctx context.Context, eventID, instanceID string) error
	GetUnhandleEventInstances(ctx context.Context, eventID string) (int64, error)
}

var _ EventInstanceDao = &sqlEventInstanceDao{}

type sqlEventInstanceDao struct {
	sessionFactory *db.SessionFactory
}

func NewEventInstanceDao(sessionFactory *db.SessionFactory) EventInstanceDao {
	return &sqlEventInstanceDao{sessionFactory: sessionFactory}
}

func (d *sqlEventInstanceDao) Get(ctx context.Context, eventID, instanceID string) (*api.EventInstance, error) {
	g2 := (*d.sessionFactory).New(ctx)
	eventInstance := api.EventInstance{}
	err := g2.Take(&eventInstance, "event_id = ? AND instance_id = ?", eventID, instanceID).Error
	if err != nil {
		return nil, err
	}

	return &eventInstance, nil
}

func (d *sqlEventInstanceDao) GetUnhandleEventInstances(ctx context.Context, eventID string) (int64, error) {
	g2 := (*d.sessionFactory).New(ctx)
	var count int64
	err := g2.Model(&api.EventInstance{}).Where("event_id = ? AND done = ?", eventID, false).Count(&count).Error
	if err != nil {
		return 0, err
	}

	return count, nil
}

func (d *sqlEventInstanceDao) Create(ctx context.Context, eventInstance *api.EventInstance) (*api.EventInstance, error) {
	g2 := (*d.sessionFactory).New(ctx)
	if err := g2.Omit(clause.Associations).Create(eventInstance).Error; err != nil {
		db.MarkForRollback(ctx, err)
		return nil, err
	}
	return eventInstance, nil
}

func (d *sqlEventInstanceDao) CreateList(ctx context.Context, eventInstanceList api.EventInstanceList) error {
	g2 := (*d.sessionFactory).New(ctx)
	if err := g2.Omit(clause.Associations).CreateInBatches(eventInstanceList, len(eventInstanceList)).Error; err != nil {
		db.MarkForRollback(ctx, err)
		return err
	}
	return nil
}

func (d *sqlEventInstanceDao) MarkAsDone(ctx context.Context, eventID, instanceID string) error {
	g2 := (*d.sessionFactory).New(ctx)
	if err := g2.Omit(clause.Associations).Table("event_instances").Where("event_id = ? AND instance_id = ?", eventID, instanceID).Update("done", true).Error; err != nil {
		db.MarkForRollback(ctx, err)
		return err
	}
	return nil
}
