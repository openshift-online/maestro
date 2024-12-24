package migrations

import (
	"gorm.io/datatypes"
	"gorm.io/gorm"

	"github.com/go-gormigrate/gormigrate/v2"
)

func addConsumers() *gormigrate.Migration {
	type Consumer struct {
		Model
		Name   string         `gorm:"uniqueIndex;not null"`
		Labels datatypes.JSON `gorm:"type:json"`
	}

	return &gormigrate.Migration{
		ID: "202311151856",
		Migrate: func(tx *gorm.DB) error {
			if err := tx.AutoMigrate(&Consumer{}); err != nil {
				return err
			}

			if err := CreateFK(tx, fkMigration{
				"resources", "consumers", "consumer_name", "consumers(name)", "ON DELETE RESTRICT ON UPDATE RESTRICT",
			}); err != nil {
				return err
			}

			return nil
		},
		Rollback: func(tx *gorm.DB) error {
			return tx.Migrator().DropTable(&Consumer{})
		},
	}
}
