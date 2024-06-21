package migrations

import (
	"gorm.io/gorm"

	"github.com/go-gormigrate/gormigrate/v2"
)

func addEventInstances() *gormigrate.Migration {
	type EventInstance struct {
		EventID    string `gorm:"index"` // primary key of events table
		InstanceID string `gorm:"index"` // primary key of server_instances table
		Done       bool
	}

	return &gormigrate.Migration{
		ID: "202406211556",
		Migrate: func(tx *gorm.DB) error {
			if err := tx.AutoMigrate(&EventInstance{}); err != nil {
				return err
			}
			// Step 1: Create the Trigger Function
			triggerFunctionSQL := `
			CREATE OR REPLACE FUNCTION check_event_instances_done()
			RETURNS TRIGGER AS $$
			BEGIN
			    RAISE NOTICE 'Checking event_id: %, undone count: %', NEW.event_id,
					(SELECT COUNT(*) FROM event_instances WHERE event_id = NEW.event_id AND done = false);
			    IF (SELECT COUNT(*) FROM event_instances WHERE event_id = NEW.event_id AND done = false) = 0 THEN
					RAISE NOTICE 'All instances done, updating reconciled_date for event_id: %', NEW.event_id;
					UPDATE events SET reconciled_date = NOW() WHERE id = NEW.event_id;
				END IF;
				RETURN NEW;
			END;
			$$ LANGUAGE plpgsql;
			`
			if err := tx.Exec(triggerFunctionSQL).Error; err != nil {
				return err
			}
			// Step 2: Create the Trigger
			triggerSQL := `
			CREATE TRIGGER trg_check_event_instances_done
			AFTER UPDATE ON event_instances
			FOR EACH ROW
			EXECUTE FUNCTION check_event_instances_done();
			`
			return tx.Exec(triggerSQL).Error
		},
		Rollback: func(tx *gorm.DB) error {
			// Rollback function to drop the trigger and trigger function
			dropTriggerSQL := `DROP TRIGGER IF EXISTS trg_check_event_instances_done ON event_instances;`
			if err := tx.Exec(dropTriggerSQL).Error; err != nil {
				return err
			}
			dropFunctionSQL := `DROP FUNCTION IF EXISTS check_event_instances_done;`
			if err := tx.Exec(dropFunctionSQL).Error; err != nil {
				return err
			}

			return tx.Migrator().DropTable(&EventInstance{})
		},
	}
}
