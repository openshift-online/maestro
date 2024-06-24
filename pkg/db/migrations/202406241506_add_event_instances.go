package migrations

import (
	"gorm.io/gorm"

	"github.com/go-gormigrate/gormigrate/v2"
)

func addEventInstances() *gormigrate.Migration {
	type EventInstance struct {
		EventID    string `gorm:"index"` // primary key of events table
		InstanceID string `gorm:"index"` // primary key of server_instances table
	}

	return &gormigrate.Migration{
		ID: "202406241506",
		Migrate: func(tx *gorm.DB) error {
			return tx.AutoMigrate(&EventInstance{})
		},
		Rollback: func(tx *gorm.DB) error {
			return tx.Migrator().DropTable(&EventInstance{})
		},
	}
}
