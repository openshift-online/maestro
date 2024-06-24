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

func (d *sqlEventInstanceDao) Create(ctx context.Context, eventInstance *api.EventInstance) (*api.EventInstance, error) {
	g2 := (*d.sessionFactory).New(ctx)
	if err := g2.Omit(clause.Associations).Create(eventInstance).Error; err != nil {
		db.MarkForRollback(ctx, err)
		return nil, err
	}
	return eventInstance, nil
}
