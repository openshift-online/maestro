package migrations

import (
	"gorm.io/gorm"

	"github.com/go-gormigrate/gormigrate/v2"
)

func addInstances() *gormigrate.Migration {
	type Instance struct {
		Model
		Name string `json:"name"`
	}

	return &gormigrate.Migration{
		ID: "202312191105",
		Migrate: func(tx *gorm.DB) error {
			return tx.AutoMigrate(&Instance{})
		},
		Rollback: func(tx *gorm.DB) error {
			return tx.Migrator().DropTable(&Instance{})
		},
	}
}
