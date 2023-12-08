package dao

import (
	"context"
	"fmt"

	"gorm.io/gorm/clause"

	"github.com/openshift-online/maestro/pkg/api"
	"github.com/openshift-online/maestro/pkg/db"
)

type EventDao interface {
	Get(ctx context.Context, id string) (*api.Event, error)
	Create(ctx context.Context, event *api.Event) (*api.Event, error)
	Replace(ctx context.Context, event *api.Event) (*api.Event, error)
	Delete(ctx context.Context, id string) error
	FindByIDs(ctx context.Context, ids []string) (api.EventList, error)
	All(ctx context.Context) (api.EventList, error)

	DeleteAllReconciledEvents(ctx context.Context) error
	FindAllUnreconciledEvents(ctx context.Context) (api.EventList, error)
}

var _ EventDao = &sqlEventDao{}

type sqlEventDao struct {
	sessionFactory *db.SessionFactory
}

func NewEventDao(sessionFactory *db.SessionFactory) EventDao {
	return &sqlEventDao{sessionFactory: sessionFactory}
}

func (d *sqlEventDao) Get(ctx context.Context, id string) (*api.Event, error) {
	g2 := (*d.sessionFactory).New(ctx)
	var event api.Event
	if err := g2.Take(&event, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &event, nil
}

func (d *sqlEventDao) Create(ctx context.Context, event *api.Event) (*api.Event, error) {
	g2 := (*d.sessionFactory).New(ctx)
	if err := g2.Omit(clause.Associations).Create(event).Error; err != nil {
		db.MarkForRollback(ctx, err)
		return nil, err
	}

	notify := fmt.Sprintf("select pg_notify('%s', '%s')", "events", event.ID)

	err := g2.Exec(notify).Error
	if err != nil {
		return nil, err
	}

	return event, nil
}

func (d *sqlEventDao) Replace(ctx context.Context, event *api.Event) (*api.Event, error) {
	g2 := (*d.sessionFactory).New(ctx)
	if err := g2.Omit(clause.Associations).Save(event).Error; err != nil {
		db.MarkForRollback(ctx, err)
		return nil, err
	}
	return event, nil
}

func (d *sqlEventDao) Delete(ctx context.Context, id string) error {
	g2 := (*d.sessionFactory).New(ctx)
	if err := g2.Unscoped().Omit(clause.Associations).Delete(&api.Event{Meta: api.Meta{ID: id}}).Error; err != nil {
		db.MarkForRollback(ctx, err)
		return err
	}
	return nil
}

func (d *sqlEventDao) DeleteAllReconciledEvents(ctx context.Context) error {
	g2 := (*d.sessionFactory).New(ctx)
	if err := g2.Unscoped().Omit(clause.Associations).Where("reconciled_date IS NOT NULL").Delete(&api.Event{}).Error; err != nil {
		db.MarkForRollback(ctx, err)
		return err
	}
	return nil
}

func (d *sqlEventDao) FindByIDs(ctx context.Context, ids []string) (api.EventList, error) {
	g2 := (*d.sessionFactory).New(ctx)
	events := api.EventList{}
	if err := g2.Where("id in (?)", ids).Find(&events).Error; err != nil {
		return nil, err
	}
	return events, nil
}

func (d *sqlEventDao) FindAllUnreconciledEvents(ctx context.Context) (api.EventList, error) {
	g2 := (*d.sessionFactory).New(ctx)
	events := api.EventList{}
	if err := g2.Where("reconciled_date IS NULL").Find(&events).Error; err != nil {
		return nil, err
	}
	return events, nil
}

func (d *sqlEventDao) All(ctx context.Context) (api.EventList, error) {
	g2 := (*d.sessionFactory).New(ctx)
	events := api.EventList{}
	if err := g2.Find(&events).Error; err != nil {
		return nil, err
	}
	return events, nil
}
