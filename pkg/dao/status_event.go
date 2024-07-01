package dao

import (
	"context"
	"fmt"

	"gorm.io/gorm/clause"

	"github.com/openshift-online/maestro/pkg/api"
	"github.com/openshift-online/maestro/pkg/db"
)

type StatusEventDao interface {
	Get(ctx context.Context, id string) (*api.StatusEvent, error)
	Create(ctx context.Context, statusEvent *api.StatusEvent) (*api.StatusEvent, error)
	Replace(ctx context.Context, statusEvent *api.StatusEvent) (*api.StatusEvent, error)
	Delete(ctx context.Context, id string) error
	FindByIDs(ctx context.Context, ids []string) (api.StatusEventList, error)
	All(ctx context.Context) (api.StatusEventList, error)

	DeleteAllReconciledEvents(ctx context.Context) error
	FindAllUnreconciledEvents(ctx context.Context) (api.StatusEventList, error)
}

var _ StatusEventDao = &sqlStatusEventDao{}

type sqlStatusEventDao struct {
	sessionFactory *db.SessionFactory
}

func NewStatusEventDao(sessionFactory *db.SessionFactory) StatusEventDao {
	return &sqlStatusEventDao{sessionFactory: sessionFactory}
}

func (d *sqlStatusEventDao) Get(ctx context.Context, id string) (*api.StatusEvent, error) {
	g2 := (*d.sessionFactory).New(ctx)
	var statusEvent api.StatusEvent
	if err := g2.Take(&statusEvent, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &statusEvent, nil
}

func (d *sqlStatusEventDao) Create(ctx context.Context, statusEvent *api.StatusEvent) (*api.StatusEvent, error) {
	g2 := (*d.sessionFactory).New(ctx)
	if err := g2.Omit(clause.Associations).Create(statusEvent).Error; err != nil {
		db.MarkForRollback(ctx, err)
		return nil, err
	}

	notify := fmt.Sprintf("select pg_notify('%s', '%s')", "status_events", statusEvent.ID)

	err := g2.Exec(notify).Error
	if err != nil {
		return nil, err
	}

	return statusEvent, nil
}

func (d *sqlStatusEventDao) Replace(ctx context.Context, statusEvent *api.StatusEvent) (*api.StatusEvent, error) {
	g2 := (*d.sessionFactory).New(ctx)
	if err := g2.Omit(clause.Associations).Save(statusEvent).Error; err != nil {
		db.MarkForRollback(ctx, err)
		return nil, err
	}
	return statusEvent, nil
}

func (d *sqlStatusEventDao) Delete(ctx context.Context, id string) error {
	g2 := (*d.sessionFactory).New(ctx)
	if err := g2.Unscoped().Omit(clause.Associations).Delete(&api.StatusEvent{Meta: api.Meta{ID: id}}).Error; err != nil {
		db.MarkForRollback(ctx, err)
		return err
	}
	return nil
}

func (d *sqlStatusEventDao) FindByIDs(ctx context.Context, ids []string) (api.StatusEventList, error) {
	g2 := (*d.sessionFactory).New(ctx)
	statusEvents := api.StatusEventList{}
	if err := g2.Where("id in (?)", ids).Find(&statusEvents).Error; err != nil {
		return nil, err
	}
	return statusEvents, nil
}

func (d *sqlStatusEventDao) DeleteAllReconciledEvents(ctx context.Context) error {
	g2 := (*d.sessionFactory).New(ctx)
	if err := g2.Unscoped().Omit(clause.Associations).Where("reconciled_date IS NOT NULL").Delete(&api.StatusEvent{}).Error; err != nil {
		db.MarkForRollback(ctx, err)
		return err
	}
	return nil
}

func (d *sqlStatusEventDao) FindAllUnreconciledEvents(ctx context.Context) (api.StatusEventList, error) {
	g2 := (*d.sessionFactory).New(ctx)
	statusEvents := api.StatusEventList{}
	if err := g2.Where("reconciled_date IS NULL").Find(&statusEvents).Error; err != nil {
		return nil, err
	}
	return statusEvents, nil
}

func (d *sqlStatusEventDao) All(ctx context.Context) (api.StatusEventList, error) {
	g2 := (*d.sessionFactory).New(ctx)
	statusEvents := api.StatusEventList{}
	if err := g2.Find(&statusEvents).Error; err != nil {
		return nil, err
	}
	return statusEvents, nil
}
