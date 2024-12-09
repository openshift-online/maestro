package migrations

import (
	"time"

	"gorm.io/datatypes"
	"gorm.io/gorm"

	"github.com/go-gormigrate/gormigrate/v2"
)

func addStatusEvents() *gormigrate.Migration {
	type StatusEvent struct {
		Model
		ResourceID      string `gorm:"index"` // resource id
		ResourceSource  string
		ResourceType    string
		Payload         datatypes.JSON `gorm:"type:json"`
		Status          datatypes.JSON `gorm:"type:json"`
		StatusEventType string         // Update|Delete, any string
		ReconciledDate  *time.Time     `gorm:"null;index"`
	}

	return &gormigrate.Migration{
		ID: "202406241426",
		Migrate: func(tx *gorm.DB) error {
			return tx.AutoMigrate(&StatusEvent{})
		},
		Rollback: func(tx *gorm.DB) error {
			return tx.Migrator().DropTable(&StatusEvent{})
		},
	}
}
