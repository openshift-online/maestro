package migrations

import (
	"github.com/go-gormigrate/gormigrate/v2"
	"gorm.io/gorm"
)

func dropDinosaurs() *gormigrate.Migration {
	type Dinosaur struct {
		Model
		Species string `gorm:"index"`
	}

	return &gormigrate.Migration{
		ID: "202311151859",
		Migrate: func(tx *gorm.DB) error {
			return tx.Migrator().DropTable(&Dinosaur{})
		},
		Rollback: func(tx *gorm.DB) error {
			return tx.AutoMigrate(&Dinosaur{})
		},
	}
}
