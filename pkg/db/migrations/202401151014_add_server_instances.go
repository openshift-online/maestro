package migrations

import (
	"gorm.io/gorm"

	"github.com/go-gormigrate/gormigrate/v2"
)

func addServerInstances() *gormigrate.Migration {
	type ServerInstance struct {
		Model
	}

	return &gormigrate.Migration{
		ID: "202401151014",
		Migrate: func(tx *gorm.DB) error {
			return tx.AutoMigrate(&ServerInstance{})
		},
		Rollback: func(tx *gorm.DB) error {
			return tx.Migrator().DropTable(&ServerInstance{})
		},
	}
}
