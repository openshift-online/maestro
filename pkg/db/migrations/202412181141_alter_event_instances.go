package migrations

import (
	"github.com/go-gormigrate/gormigrate/v2"
	"gorm.io/gorm"
)

func alterEventInstances() *gormigrate.Migration {
	type EventInstance struct {
		EventID     string `gorm:"index:idx_status_event_instance"` // primary key of status_events table
		InstanceID  string `gorm:"index:idx_status_event_instance"` // primary key of server_instances table
		SpecEventID string `gorm:"index"`                           // primary key of events table
	}

	return &gormigrate.Migration{
		ID: "202412181141",
		Migrate: func(tx *gorm.DB) error {
			if err := tx.AutoMigrate(&EventInstance{}); err != nil {
				return err
			}

			return CreateFK(tx, fkMigration{
				"event_instances", "server_instances", "instance_id", "server_instances(id)", "ON DELETE CASCADE",
			}, fkMigration{
				"event_instances", "status_events", "event_id", "status_events(id)", "ON DELETE CASCADE",
			}, fkMigration{
				"event_instances", "events", "spec_event_id", "events(id)", "ON DELETE CASCADE",
			})
		},
		Rollback: func(tx *gorm.DB) error {
			if err := tx.Migrator().DropColumn(&EventInstance{}, "spec_event_id"); err != nil {
				return err
			}

			if err := tx.Migrator().DropIndex(&EventInstance{}, "idx_status_event_instance"); err != nil {
				return err
			}

			if err := tx.Migrator().DropConstraint(&EventInstance{}, fkName("event_instances", "server_instances")); err != nil {
				return err
			}

			if err := tx.Migrator().DropConstraint(&EventInstance{}, fkName("event_instances", "status_events")); err != nil {
				return err
			}

			return tx.Migrator().DropConstraint(&EventInstance{}, fkName("event_instances", "events"))
		},
	}
}
