package migrations

import (
	"gorm.io/datatypes"
	"gorm.io/gorm"

	"github.com/go-gormigrate/gormigrate/v2"
)

func addConsumers() *gormigrate.Migration {
	type Consumer struct {
		Model
		Labels datatypes.JSON `gorm:"type:json"`
	}

	return &gormigrate.Migration{
		ID: "202311151856",
		Migrate: func(tx *gorm.DB) error {
			return tx.AutoMigrate(&Consumer{})
		},
		Rollback: func(tx *gorm.DB) error {
			return tx.Migrator().DropTable(&Consumer{})
		},
	}
}
