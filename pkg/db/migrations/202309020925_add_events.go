package migrations

import (
	"time"

	"gorm.io/gorm"

	"github.com/go-gormigrate/gormigrate/v2"
)

func addEvents() *gormigrate.Migration {
	type Event struct {
		Model
		Source string `gorm:"index"` // MyTable, any string
		// SourceID must be an indexable key for querying, *not* a json data payload.
		// an indexed column of data json data would explode
		SourceID       string     `gorm:"index"` // primary key of MyTable
		EventType      string     // Add|Update|Delete, any string
		ReconciledDate *time.Time `gorm:"null;index"`
	}

	return &gormigrate.Migration{
		ID: "202309020925",
		Migrate: func(tx *gorm.DB) error {
			return tx.AutoMigrate(&Event{})
		},
		Rollback: func(tx *gorm.DB) error {
			return tx.Migrator().DropTable(&Event{})
		},
	}
}
