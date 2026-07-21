package dao

import (
	"context"
	"fmt"
	"time"

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
	FindAgeOfOldestUnreconciledEvent(ctx context.Context) (*float64, error)
	ReconcileStaleDeleteEvents(ctx context.Context, cutoff time.Time) (int64, error)
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

// StaleDeleteReconcileBatchSize bounds how many stale delete events a single
// ReconcileStaleDeleteEvents call retires, so one periodic tick can never issue an
// unbounded UPDATE (a backlog can reach hundreds of thousands of rows). Any remainder
// is drained by subsequent ticks of the periodic detector.
const StaleDeleteReconcileBatchSize = 10000

// ReconcileStaleDeleteEvents marks as reconciled every unreconciled delete event whose
// resource has been soft-deleted before the given cutoff. Such delete events are stuck:
// the resource is soft-deleted (so PredicateEvent finds it via the Unscoped read and never
// takes the 404 mark-reconciled fast path) but its agent is gone (never acknowledged the
// delete), so the spec-event worker skips the event on every pass and the periodic sync
// re-enqueues it forever. Retiring them stops that starvation loop. Each call retires at
// most StaleDeleteReconcileBatchSize events to keep a single tick bounded, and returns the
// number of events reconciled.
func (d *sqlEventDao) ReconcileStaleDeleteEvents(ctx context.Context, cutoff time.Time) (int64, error) {
	staleResourceIDs := (*d.sessionFactory).New(ctx).
		Unscoped().
		Model(&api.Resource{}).
		Select("id").
		Where("deleted_at IS NOT NULL AND deleted_at < ?", cutoff)

	staleEventIDs := (*d.sessionFactory).New(ctx).
		Model(&api.Event{}).
		Select("id").
		Where("source = ? AND event_type = ? AND reconciled_date IS NULL", "Resources", api.DeleteEventType).
		Where("source_id IN (?)", staleResourceIDs).
		Limit(StaleDeleteReconcileBatchSize)

	result := (*d.sessionFactory).New(ctx).
		Model(&api.Event{}).
		Where("id IN (?) AND reconciled_date IS NULL", staleEventIDs).
		Update("reconciled_date", time.Now())
	if result.Error != nil {
		db.MarkForRollback(ctx, result.Error)
		return 0, result.Error
	}
	return result.RowsAffected, nil
}

func (d *sqlEventDao) All(ctx context.Context) (api.EventList, error) {
	g2 := (*d.sessionFactory).New(ctx)
	events := api.EventList{}
	if err := g2.Find(&events).Error; err != nil {
		return nil, err
	}
	return events, nil
}

func (d *sqlEventDao) FindAgeOfOldestUnreconciledEvent(ctx context.Context) (*float64, error) {
	g2 := (*d.sessionFactory).New(ctx)
	var ageSeconds *float64
	result := g2.Raw("SELECT EXTRACT(EPOCH FROM now() - MIN(created_at)) FROM events WHERE reconciled_date IS NULL").
		Scan(&ageSeconds)
	if result.Error != nil {
		return nil, result.Error
	}

	return ageSeconds, nil
}
