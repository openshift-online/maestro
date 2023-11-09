# Migrations

Database migrations are handled by this package. All migrations should be created in separate files, following a starndard naming convetion

The `migrations.go` file defines an array of migrate functions that are called by the [gormigrate](https://gopkg.in/gormigrate.v1) helper. Each migration function should perform a specific migration.

## Creating a new migration

Create a migration ID based on the time using the YYYYMMDDHHMM  format. Example: `August 21 2018 at 2:54pm` would be `201808211454`.

Your migration's name should be used in the file name and in the function name and should adequately represent the actions your migration is taking. If your migration is doing too much to fit in a name, you should consider creating multiple migrations.

Create a separate file in `pkg/db/` following the naming schema in place: `<migration_id>_<migration_name>.go`. In the file, you'll create a function that returns a [gormmigrate.Migration](https://gopkg.in/gormigrate.v1/blob/master/gormigrate.go#L37) object with `gormigrate.Migrate` and `gormigrate.Rollback` functions defined.

Add the function you created in the separate file to the `migrations` list in `pkg/db/migrations.go`.

If necessary, write a test to verify the migration. See `test/integration/migrations_test.go` for examples.

## Migration Rules

### Migration IDs

Each migration has an ID that defines the order in which the migration is run.

IDs are numerical timestamps that must sort ascending. Use YYYYMMDDHHMM w/ 24 hour time for format.
Example: `August 21 2018 at 2:54pm` would be `201808211454`.

Migration IDs must be descending. If you create a migration, submit an MR, and another MR is merged before yours is able to be merged, you must update the ID to represent a date later than any previous migration.

### Models in Migrations

Represent modesl inline with migrations to represent the evolution of the object over time. 

For example, it is necessary to add a boolean field "hidden" to the "Account" model. This is how you would represent the account model in that migration:
```golang
type Account struct {
  Model
  Username string
  FirstName string
  LastName string
  Hidden boolean
}

err := tx.AutoMigrate(&Account{}).Error
if err != nil {
...
```

**DO NOT IMPORT THE API PKG**. When a migration imports the `api` pkg and uses models defined in it, the migration may work the first time it is run. The models in `pkg/api` are bound to change as the project grows. Eventually, the models could change so that the migration breaks, causing any new deployments to fail on your old shitty migration.

### Record Deletions

If it is necessary to delete a record in a migration, be aware of a couple caveats around deleting wth gorm:

1. You must pass a record with an ID to `gorm.Delete(&record)`, otherwise **ALL RECORDS WILL BE DELETED**

2. Gorm [soft deletes](http://gorm.io/docs/delete.html#Soft-Delete) by default. This means it sets the `deleted_at` field to a non-null value and any subsequent gorm calls will ignore it. If you are deleting a record that needs to be permanently deleted (like permissions), use `gorm.Unscoped().Delete`.

See the [gorm documentation around deletions](http://gorm.io/docs/delete.html) for more information

## Migration tests

In most cases, it shouldn't be necessary to create a test for a migration. However, if the migration is manipulating records and poses a significant risk of completely borking up important data, a test should be written.

Tests are difficult to write for migrations and are likely to fail one day long after the migration has already run in production. After a migration is run in production, it is safe to delete the test from the integration test suite.

The test `helper` has a couple helpful functions for testing migrations. You can use `h.CleanDB()` to completely wipe the database clean, then `h.MigrateTo(<migration_id>)` to migrate to a specific migration ID. You should then be able to create whatever records in the database you need to test against and finally run `h.MigrateDB()` to run your created migration and all subsequent migrations.
