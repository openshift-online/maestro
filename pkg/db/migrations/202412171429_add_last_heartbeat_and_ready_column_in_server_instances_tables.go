package migrations

import (
	"time"

	"gorm.io/gorm"

	"github.com/go-gormigrate/gormigrate/v2"
)

func addLastHeartBeatAndReadyColumnInServerInstancesTable() *gormigrate.Migration {
	type ServerInstance struct {
		LastHeartbeat time.Time
		Ready         bool `gorm:"default:false"`
	}

	return &gormigrate.Migration{
		ID: "202412171429",
		Migrate: func(tx *gorm.DB) error {
			return tx.AutoMigrate(&ServerInstance{})
		},
		Rollback: func(tx *gorm.DB) error {
			err := tx.Migrator().DropColumn(&ServerInstance{}, "ready")
			if err != nil {
				return err
			}
			return tx.Migrator().DropColumn(&ServerInstance{}, "last_heartbeat")
		},
	}
}
