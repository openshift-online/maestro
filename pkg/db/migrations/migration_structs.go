package migrations

import (
	"context"
	"fmt"
	"time"

	"gorm.io/gorm"

	"github.com/go-gormigrate/gormigrate/v2"
	"github.com/openshift-online/maestro/pkg/logger"
)

var log = logger.NewOCMLogger(context.Background())

// gormigrate is a wrapper for gorm's migration functions that adds schema versioning and rollback capabilities.
// For help writing migration steps, see the gorm documentation on migrations: http://doc.gorm.io/database.html#migration

// MigrationList rules:
//
//  1. IDs are numerical timestamps that must sort ascending.
//     Use YYYYMMDDHHMM w/ 24 hour time for format
//     Example: August 21 2018 at 2:54pm would be 201808211454.
//
//  2. Include models inline with migrations to see the evolution of the object over time.
//     Using our internal type models directly in the first migration would fail in future clean installs.
//
//  3. Migrations must be backwards compatible. There are no new required fields allowed.
//     See $project_home/g2/README.md
//
// 4. Create one function in a separate file that returns your Migration. Add that single function call to this list.
var MigrationList = []*gormigrate.Migration{
	addDinosaurs(),
	addEvents(),
	addResources(),
	addConsumers(),
	dropDinosaurs(),
	addServerInstances(),
	addStatusEvents(),
	addEventInstances(),
	addLastHeartBeatAndReadyColumnInServerInstancesTable(),
	alterEventInstances(),
}

// CleanUpDirtyData clean up the dirty data before migrating the tables.
// Note: when new constraints will be added to old tables, we should especially consider the possibility of dirty data.
func CleanUpDirtyData(db *gorm.DB) error {
	//TODO: cleanup dirty data before add new constraints to old tables
	return nil
}

// Model represents the base model struct. All entities will have this struct embedded.
type Model struct {
	ID        string `gorm:"primary_key"`
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt gorm.DeletedAt `gorm:"index"`
}

type fkMigration struct {
	Model      string
	Dest       string
	Field      string
	Reference  string
	Constraint string
}

func CreateFK(g2 *gorm.DB, fks ...fkMigration) error {
	var drop = `ALTER TABLE %s DROP CONSTRAINT IF EXISTS %s;`

	for _, fk := range fks {
		name := fkName(fk.Model, fk.Dest)

		g2.Exec(fmt.Sprintf(drop, fk.Model, name))
		if err := g2.Exec(fmt.Sprintf(`ALTER TABLE %s ADD CONSTRAINT %s FOREIGN KEY (%s) REFERENCES %s %s;`,
			fk.Model, name, fk.Field, fk.Reference, fk.Constraint)).Error; err != nil {
			return err
		}
	}
	return nil
}

func fkName(model, dest string) string {
	return fmt.Sprintf("fk_%s_%s", model, dest)
}
