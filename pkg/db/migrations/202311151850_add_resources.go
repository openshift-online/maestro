package migrations

import (
	"gorm.io/datatypes"
	"gorm.io/gorm"

	"github.com/go-gormigrate/gormigrate/v2"
)

func addResources() *gormigrate.Migration {
	type Resource struct {
		Model
		ConsumerID      string         `gorm:"index"`
		Version         int            `gorm:"not null"`
		ObservedVersion int            `gorm:"not null"`
		Manifest        datatypes.JSON `gorm:"type:json"`
		Status          datatypes.JSON `gorm:"type:json"`
	}

	return &gormigrate.Migration{
		ID: "202311151850",
		Migrate: func(tx *gorm.DB) error {
			return tx.AutoMigrate(&Resource{})
		},
		Rollback: func(tx *gorm.DB) error {
			return tx.Migrator().DropTable(&Resource{})
		},
	}
}
