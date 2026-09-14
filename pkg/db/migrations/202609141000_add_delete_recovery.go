package migrations

import (
	"github.com/go-gormigrate/gormigrate/v2"
	"gorm.io/gorm"
)

// addDeleteRecovery adds durable retry scheduling and retryable concurrent indexes.
func addDeleteRecovery() *gormigrate.Migration {
	return &gormigrate.Migration{
		ID: "202609141000",
		Migrate: func(tx *gorm.DB) error {
			// Nullable columns avoid rewriting or eagerly scheduling the tombstone
			// backlog. The scheduler initializes it in bounded batches.
			if err := tx.Transaction(func(tx *gorm.DB) error {
				return tx.Exec(`
					SET LOCAL lock_timeout = '5s';
					ALTER TABLE resources ADD COLUMN IF NOT EXISTS delete_retry_at timestamptz;
					ALTER TABLE resources ADD COLUMN IF NOT EXISTS delete_retry_delay bigint;
					CREATE TABLE IF NOT EXISTS delete_recovery_consumers (
						consumer_name text PRIMARY KEY REFERENCES consumers(name) ON DELETE CASCADE,
						next_at timestamptz NOT NULL
					);
					CREATE INDEX IF NOT EXISTS idx_delete_recovery_consumers_due
						ON delete_recovery_consumers (next_at, consumer_name);
					CREATE TABLE IF NOT EXISTS delete_recovery_schedule (
						id integer PRIMARY KEY CHECK (id = 1),
						next_at timestamptz NOT NULL
					);
					INSERT INTO delete_recovery_schedule (id, next_at)
						VALUES (1, clock_timestamp()) ON CONFLICT DO NOTHING;
				`).Error
			}); err != nil {
				return err
			}
			for _, index := range []struct{ drop, create string }{
				{
					"DROP INDEX CONCURRENTLY IF EXISTS idx_resources_delete_recovery_bootstrap",
					`CREATE INDEX CONCURRENTLY idx_resources_delete_recovery_bootstrap ON resources (deleted_at, id)
						WHERE deleted_at IS NOT NULL AND delete_retry_at IS NULL`,
				},
				{
					"DROP INDEX CONCURRENTLY IF EXISTS idx_resources_delete_recovery_due",
					`CREATE INDEX CONCURRENTLY idx_resources_delete_recovery_due ON resources (consumer_name, delete_retry_at, id)
						WHERE deleted_at IS NOT NULL AND delete_retry_at IS NOT NULL`,
				},
				// Non-unique: an upgraded database can contain duplicate legacy
				// events. Coalescing uses the resource row lock, not a new constraint.
				{
					"DROP INDEX CONCURRENTLY IF EXISTS idx_events_pending_resource_delete",
					`CREATE INDEX CONCURRENTLY idx_events_pending_resource_delete ON events (source_id)
						WHERE source = 'Resources' AND event_type = 'Delete' AND reconciled_date IS NULL`,
				},
			} {
				// Retry interrupted concurrent builds, including invalid indexes.
				if err := tx.Exec(index.drop).Error; err != nil {
					return err
				}
				if err := tx.Exec(index.create).Error; err != nil {
					return err
				}
			}
			return nil
		},
		Rollback: func(tx *gorm.DB) error {
			return tx.Transaction(func(tx *gorm.DB) error {
				return tx.Exec(`
					SET LOCAL lock_timeout = '5s';
					DROP TABLE IF EXISTS delete_recovery_schedule;
					DROP TABLE IF EXISTS delete_recovery_consumers;
					DROP INDEX IF EXISTS idx_events_pending_resource_delete;
					ALTER TABLE resources DROP COLUMN IF EXISTS delete_retry_at;
					ALTER TABLE resources DROP COLUMN IF EXISTS delete_retry_delay;
				`).Error
			})
		},
	}
}
