package migrations

import (
	"gorm.io/datatypes"
	"gorm.io/gorm"

	"github.com/go-gormigrate/gormigrate/v2"
)

func addResources() *gormigrate.Migration {
	type Resource struct {
		Model
		Source       string `gorm:"index"`
		ConsumerName string `gorm:"index"`
		Version      int    `gorm:"not null"`
		// Type indicates the resource type. Supported types: "Single" and "Bundle".
		// "Single" resource type for RESTful API calls,
		// "Bundle" resource type mainly for gRPC calls.
		Type string `gorm:"index"`
		// Manifest holds the resource manifest in CloudEvent format (JSON representation).
		Manifest datatypes.JSON `gorm:"type:json"`
		// Status represents the resource status in CloudEvent format (JSON representation).
		Status datatypes.JSON `gorm:"type:json"`
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
