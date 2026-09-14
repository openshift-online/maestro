package dao

import (
	"context"
	"math/rand/v2"
	"time"

	"gorm.io/gorm"

	"github.com/openshift-online/maestro/pkg/api"
	"github.com/openshift-online/maestro/pkg/config"
	"github.com/openshift-online/maestro/pkg/db"
)

// DeleteRecoveryRoundInterval is shared by all replicas through a durable
// database deadline. Missed rounds do not accumulate credits.
const DeleteRecoveryRoundInterval = time.Second

type DeleteRecovery struct {
	sessions db.SessionFactory
	initial  time.Duration
	maximum  time.Duration
	batch    int
}

type DeleteRecoveryResult struct {
	// Claimed is true only when this call claimed and committed the fleet round.
	// Errors return a zero result, including any work rolled back.
	Claimed     bool
	Initialized int
	Consumers   int
	Published   int
}

func NewDeleteRecovery(sessions db.SessionFactory, c *config.EventServerConfig) (*DeleteRecovery, error) {
	if err := c.ValidateDeleteRecovery(); err != nil {
		return nil, err
	}
	return &DeleteRecovery{
		sessions: sessions,
		initial:  time.Duration(c.DeleteEventRepublishInterval) * time.Second,
		maximum:  time.Duration(c.DeleteEventRepublishMaxInterval) * time.Second,
		batch:    c.DeleteEventRepublishBatchSize,
	}, nil
}

type deletionRetry struct {
	ID               string
	ConsumerName     string
	DeletedAt        time.Time
	DeleteRetryAt    time.Time
	DeleteRetryDelay time.Duration
}

// Run commits one bounded fleet-wide round, including publication notifications,
// resource backoff and the next round deadline. It never deletes a resource.
func (d *DeleteRecovery) Run(ctx context.Context) (DeleteRecoveryResult, error) {
	result := DeleteRecoveryResult{}
	if d.initial == 0 {
		return result, nil
	}
	err := d.sessions.New(ctx).Transaction(func(tx *gorm.DB) error {
		var claimed []int
		if err := tx.Raw(`SELECT id FROM delete_recovery_schedule
			WHERE id = 1 AND next_at <= clock_timestamp() FOR UPDATE SKIP LOCKED`).
			Scan(&claimed).Error; err != nil {
			return err
		}
		if len(claimed) == 0 {
			return nil
		}
		result.Claimed = true
		var now time.Time
		if err := tx.Raw("SELECT clock_timestamp()").Scan(&now).Error; err != nil {
			return err
		}
		if err := d.initialize(tx, now, &result); err != nil {
			return err
		}
		var consumers []string
		if err := tx.Raw(`SELECT consumer_name FROM delete_recovery_consumers
			WHERE next_at <= ? ORDER BY next_at, consumer_name LIMIT ?`, now, d.batch).
			Scan(&consumers).Error; err != nil {
			return err
		}
		for _, consumer := range consumers {
			result.Consumers++
			if err := d.recoverConsumer(tx, consumer, now, &result); err != nil {
				return err
			}
		}
		// Use the end of the round, not its start. Slow rounds or additional
		// replicas must not permit consecutive catch-up bursts.
		return tx.Exec(`UPDATE delete_recovery_schedule
			SET next_at = clock_timestamp() + interval '1 second' WHERE id = 1`).Error
	})
	if err != nil {
		return DeleteRecoveryResult{}, err
	}
	return result, nil
}

func (d *DeleteRecovery) initialize(tx *gorm.DB, now time.Time, result *DeleteRecoveryResult) error {
	var ids []string
	if err := tx.Raw(`SELECT id FROM resources
		WHERE deleted_at IS NOT NULL AND delete_retry_at IS NULL
		ORDER BY deleted_at, id LIMIT ?`, d.batch).Scan(&ids).Error; err != nil {
		return err
	}
	for _, id := range ids {
		var rows []deletionRetry
		// Limit candidates before skipping locks, so a busy table cannot turn
		// a bounded bootstrap into a scan of every locked tombstone.
		if err := tx.Raw(`SELECT id, consumer_name, deleted_at FROM resources
			WHERE id = ? AND deleted_at IS NOT NULL AND delete_retry_at IS NULL
			FOR UPDATE SKIP LOCKED`, id).Scan(&rows).Error; err != nil {
			return err
		}
		if len(rows) == 0 {
			continue
		}
		r := rows[0]
		due := r.DeletedAt.Add(d.initial)
		if err := tx.Exec(`UPDATE resources SET delete_retry_at = ?, delete_retry_delay = ?
			WHERE id = ?`, due, int64(d.initial), id).Error; err != nil {
			return err
		}
		// Do not move a queued consumer backwards when more of its backlog is
		// initialized. Newly initialized consumers also join behind those
		// already waiting, regardless of their original deletion ages.
		if err := tx.Exec(`INSERT INTO delete_recovery_consumers (consumer_name, next_at)
			VALUES (?, GREATEST(?::timestamptz, ?::timestamptz)) ON CONFLICT (consumer_name) DO UPDATE
			SET next_at = LEAST(delete_recovery_consumers.next_at, EXCLUDED.next_at)`,
			r.ConsumerName, due, now).Error; err != nil {
			return err
		}
		result.Initialized++
	}
	return nil
}

func (d *DeleteRecovery) recoverConsumer(tx *gorm.DB, consumer string, now time.Time, result *DeleteRecoveryResult) error {
	var ids []string
	if err := tx.Raw(`SELECT id FROM resources WHERE consumer_name = ?
		AND deleted_at IS NOT NULL AND delete_retry_at IS NOT NULL AND delete_retry_at <= ?
		ORDER BY delete_retry_at, id LIMIT 1`, consumer, now).Scan(&ids).Error; err != nil {
		return err
	}
	if len(ids) != 0 {
		var rows []deletionRetry
		if err := tx.Raw(`SELECT id, delete_retry_delay FROM resources
			WHERE id = ? AND deleted_at IS NOT NULL FOR UPDATE SKIP LOCKED`, ids[0]).
			Scan(&rows).Error; err != nil {
			return err
		}
		if len(rows) != 0 {
			r := rows[0]
			published, err := createResourceDeleteEvent(tx, &api.Event{
				Source: "Resources", SourceID: r.ID, EventType: api.DeleteEventType,
			})
			if err != nil {
				return err
			}
			delay := d.initial
			nominal := r.DeleteRetryDelay
			if published {
				result.Published++
				nominal = nextDeleteRetryDelay(nominal, d.initial, d.maximum)
				delay = jitterDeleteRetryDelay(nominal)
			}
			if err := tx.Exec(`UPDATE resources SET delete_retry_at = ?, delete_retry_delay = ?
				WHERE id = ?`, now.Add(delay), int64(nominal), r.ID).Error; err != nil {
				return err
			}
		}
	}
	var due []time.Time
	if err := tx.Raw(`SELECT delete_retry_at FROM resources WHERE consumer_name = ?
		AND deleted_at IS NOT NULL AND delete_retry_at IS NOT NULL
		ORDER BY delete_retry_at, id LIMIT 1`, consumer).Scan(&due).Error; err != nil {
		return err
	}
	if len(due) == 0 {
		return tx.Exec("DELETE FROM delete_recovery_consumers WHERE consumer_name = ?", consumer).Error
	}
	next := due[0]
	if next.Before(now) {
		next = now
	}
	return tx.Exec("UPDATE delete_recovery_consumers SET next_at = ? WHERE consumer_name = ?", next, consumer).Error
}

func nextDeleteRetryDelay(previous, initial, maximum time.Duration) time.Duration {
	if previous < initial {
		previous = initial
	}
	if previous >= maximum/2 {
		return maximum
	}
	return previous * 2
}

func jitterDeleteRetryDelay(nominal time.Duration) time.Duration {
	// Equal jitter, [nominal/2, nominal), retains a positive lower bound and
	// cannot overflow or exceed the configured maximum.
	half := nominal / 2
	return half + time.Duration(rand.Int64N(int64(nominal-half)))
}
