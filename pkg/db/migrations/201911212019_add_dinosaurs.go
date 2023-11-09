package migrations

// Migrations should NEVER use types from other packages. Types can change
// and then migrations run on a _new_ database will fail or behave unexpectedly.
// Instead of importing types, always re-create the type in the migration, as
// is done here, even though the same type is defined in pkg/api

import (
	"gorm.io/gorm"

	"github.com/go-gormigrate/gormigrate/v2"
)

func addDinosaurs() *gormigrate.Migration {
	type Dinosaur struct {
		Model
		Species string `gorm:"index"`
	}

	return &gormigrate.Migration{
		ID: "201911212019",
		Migrate: func(tx *gorm.DB) error {
			return tx.AutoMigrate(&Dinosaur{})
		},
		Rollback: func(tx *gorm.DB) error {
			return tx.Migrator().DropTable(&Dinosaur{})
		},
	}
}
