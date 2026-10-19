package dao

import (
	"context"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/openshift-online/maestro/pkg/api"
	"github.com/openshift-online/maestro/pkg/db"
)

// ErrConsumerCascadeBlocked means at least one resource is live or too recent.
var ErrConsumerCascadeBlocked = errors.New("consumer has live or recently soft-deleted resources")

// ConsumerCascadeEventBuilder builds the terminal event while resource locks are held.
type ConsumerCascadeEventBuilder func(*api.Resource) (*api.StatusEvent, error)

// ConsumerCascadeDao atomically finalizes eligible resources and their consumer, and
// terminal resource acknowledgements. All methods take the consumer advisory lock on
// the transaction that performs the protected reads and writes, so a lock holder never
// waits for a second pooled connection.
type ConsumerCascadeDao interface {
	Delete(context.Context, string, string, time.Duration, ConsumerCascadeEventBuilder) (api.ResourceList, error)
	// FinalizeTerminalAck commits the terminal status event and resource removal for an
	// agent delete acknowledgement in one transaction, guarded by the consumer advisory
	// lock taken on that transaction.
	FinalizeTerminalAck(context.Context, string, string, ConsumerCascadeEventBuilder) (bool, error)
}

type sqlConsumerCascadeDao struct {
	sessions db.SessionFactory
}

// NewConsumerCascadeDao constructs the atomic consumer/resource finalization store.
func NewConsumerCascadeDao(sessions db.SessionFactory) ConsumerCascadeDao {
	return &sqlConsumerCascadeDao{sessions: sessions}
}

// Delete commits resource finalization, terminal events, notifications and consumer removal together.
// The consumer advisory lock is acquired on this transaction and released at commit: the
// lock holder performs every protected statement on the lock's own connection. The lock is
// taken before any row locks so concurrent cascades cannot deadlock on the consumer row.
func (d *sqlConsumerCascadeDao) Delete(ctx context.Context, id, consumerName string, threshold time.Duration,
	buildEvent ConsumerCascadeEventBuilder) (api.ResourceList, error) {
	if threshold <= 0 {
		return nil, ErrConsumerCascadeBlocked
	}
	var finalized api.ResourceList
	err := d.sessions.New(ctx).Transaction(func(tx *gorm.DB) error {
		if err := db.LockInTx(tx, consumerName, db.Consumers); err != nil {
			return err
		}
		var consumer api.Consumer
		// NO KEY UPDATE permits the scheduler's FK checks while it holds resource
		// locks, avoiding an inverted consumer/resource lock order. The final FK
		// restriction still rejects a concurrent new resource and rolls us back.
		err := tx.Unscoped().Clauses(clause.Locking{Strength: "NO KEY UPDATE"}).Take(&consumer, "id = ?", id).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		if err != nil {
			return err
		}
		var resources api.ResourceList
		if err := tx.Unscoped().Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("consumer_name = ?", consumer.Name).Order("id").Find(&resources).Error; err != nil {
			return err
		}
		var now time.Time
		if err := tx.Raw("SELECT clock_timestamp()").Scan(&now).Error; err != nil {
			return err
		}
		cutoff := now.Add(-threshold)
		for _, resource := range resources {
			if !resource.DeletedAt.Valid || !resource.DeletedAt.Time.Before(cutoff) {
				return ErrConsumerCascadeBlocked
			}
		}
		for _, resource := range resources {
			statusEvent, err := buildEvent(resource)
			if err != nil {
				return err
			}
			if err := tx.Unscoped().Omit(clause.Associations).Delete(resource).Error; err != nil {
				return err
			}
			if err := tx.Model(&api.Event{}).
				Where("source = ? AND source_id = ? AND event_type = ? AND reconciled_date IS NULL",
					"Resources", resource.ID, api.DeleteEventType).
				Update("reconciled_date", now).Error; err != nil {
				return err
			}
			if err := tx.Omit(clause.Associations).Create(statusEvent).Error; err != nil {
				return err
			}
			// PostgreSQL delivers NOTIFY only after commit. A failure rolls back the
			// resource deletion and event together, so no notification gap is left.
			if err := tx.Exec("SELECT pg_notify(?, ?)", "status_events", statusEvent.ID).Error; err != nil {
				return err
			}
		}
		if err := tx.Unscoped().Omit(clause.Associations).Delete(&consumer).Error; err != nil {
			return err
		}
		finalized = resources
		return nil
	})
	if err != nil {
		return nil, err
	}
	return finalized, nil
}

// FinalizeTerminalAck rechecks an agent-acknowledged resource under the consumer advisory
// lock and, when it still exists, commits its terminal status event, the status notification
// and its removal together. The advisory lock is taken on this transaction, so concurrent
// terminal acknowledgements and cascades for one consumer serialize without any of them
// holding an extra pooled connection. It reports false when the resource is already gone,
// for example when a cascade or another acknowledgement finalized it first.
func (d *sqlConsumerCascadeDao) FinalizeTerminalAck(ctx context.Context, resourceID, consumerName string,
	buildEvent ConsumerCascadeEventBuilder) (bool, error) {
	finalized := false
	err := d.sessions.New(ctx).Transaction(func(tx *gorm.DB) error {
		if err := db.LockInTx(tx, consumerName, db.Consumers); err != nil {
			return err
		}
		var found api.Resource
		err := tx.Unscoped().Take(&found, "id = ?", resourceID).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		if err != nil {
			return err
		}
		if found.ConsumerName != consumerName {
			return fmt.Errorf("unmatched consumer name %s for resource %s", consumerName, resourceID)
		}
		statusEvent, err := buildEvent(&found)
		if err != nil {
			return err
		}
		if err := tx.Omit(clause.Associations).Create(statusEvent).Error; err != nil {
			return err
		}
		if err := tx.Exec("SELECT pg_notify(?, ?)", "status_events", statusEvent.ID).Error; err != nil {
			return err
		}
		if err := tx.Unscoped().Omit(clause.Associations).Delete(&api.Resource{Meta: api.Meta{ID: resourceID}}).Error; err != nil {
			return err
		}
		finalized = true
		return nil
	})
	return finalized, err
}
