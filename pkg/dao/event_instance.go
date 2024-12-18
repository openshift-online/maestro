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
	GetInstancesBySpecEventID(ctx context.Context, eventID string) ([]string, error)
	FindEventInstancesByEventIDs(ctx context.Context, ids []string) (api.EventInstanceList, error)
	GetEventsAssociatedWithInstances(ctx context.Context, instanceIDs []string) ([]string, error)
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

func (d *sqlEventInstanceDao) GetInstancesBySpecEventID(ctx context.Context, specEventID string) ([]string, error) {
	g2 := (*d.sessionFactory).New(ctx)
	var eventInstances []api.EventInstance
	if err := g2.Model(&api.EventInstance{}).Where("spec_event_id = ?", specEventID).Find(&eventInstances).Error; err != nil {
		return nil, err
	}
	instanceIDs := make([]string, len(eventInstances))
	for i, eventInstance := range eventInstances {
		instanceIDs[i] = eventInstance.InstanceID
	}
	return instanceIDs, nil
}

func (d *sqlEventInstanceDao) FindEventInstancesByEventIDs(ctx context.Context, ids []string) (api.EventInstanceList, error) {
	g2 := (*d.sessionFactory).New(ctx)
	eventInstances := api.EventInstanceList{}
	if err := g2.Where("event_id in (?)", ids).Find(&eventInstances).Error; err != nil {
		return nil, err
	}
	return eventInstances, nil
}

func (d *sqlEventInstanceDao) GetEventsAssociatedWithInstances(ctx context.Context, instanceIDs []string) ([]string, error) {
	var eventIDs []string

	instanceCount := len(instanceIDs)
	if instanceCount == 0 {
		return eventIDs, nil
	}

	g2 := (*d.sessionFactory).New(ctx)

	// Currently, the instance table should be small, if the instance table become to large,
	// consider using join to optimize
	if err := g2.Table("event_instances").
		Select("event_id").
		Where("instance_id IN (?) AND event_id IS NOT NULL", instanceIDs).
		Group("event_id").
		Having("COUNT(DISTINCT instance_id) = ?", instanceCount).
		Scan(&eventIDs).Error; err != nil {
		return nil, err
	}

	return eventIDs, nil
}
