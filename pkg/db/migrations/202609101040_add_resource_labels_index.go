package migrations

import (
	"github.com/go-gormigrate/gormigrate/v2"
	"gorm.io/gorm"
)

func addResourceLabelsIndex() *gormigrate.Migration {
	return &gormigrate.Migration{
		ID: "202609101040",
		Migrate: func(tx *gorm.DB) error {
			// Concurrent builds can leave an invalid index if interrupted. Rebuild it
			// on retry rather than skipping it with CREATE INDEX IF NOT EXISTS.
			if err := tx.Exec("DROP INDEX CONCURRENTLY IF EXISTS idx_resources_payload_labels").Error; err != nil {
				return err
			}

			// Gormigrate runs without a transaction, as required by CONCURRENTLY.
			return tx.Exec(`CREATE INDEX CONCURRENTLY idx_resources_payload_labels
				ON resources USING gin ((payload -> 'metadata' -> 'labels') jsonb_path_ops)`).Error
		},
		Rollback: func(tx *gorm.DB) error {
			return tx.Exec("DROP INDEX CONCURRENTLY IF EXISTS idx_resources_payload_labels").Error
		},
	}
}
