package db

import (
	"context"
	"database/sql/driver"
	"errors"
	"time"

	"github.com/go-gormigrate/gormigrate/v2"
	"gorm.io/gorm"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/klog/v2"

	"github.com/openshift-online/maestro/pkg/db/migrations"
)

// gormigrate is a wrapper for gorm's migration functions that adds schema versioning and rollback capabilities.
// For help writing migration steps, see the gorm documentation on migrations: http://doc.gorm.io/database.html#migration

func Migrate(g2 *gorm.DB) (result error) {
	sqlDB, err := g2.DB()
	if err != nil {
		return err
	}
	conn, err := sqlDB.Conn(g2.Statement.Context)
	if err != nil {
		return err
	}
	defer func() {
		// Release the session lock even if the migration's context was cancelled.
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if _, err := conn.ExecContext(ctx, "SELECT pg_advisory_unlock(hashtext('maestro'), hashtext('migrations'))"); err != nil {
			result = errors.Join(result, err)
			// Never return a connection that may still hold the lock to the pool.
			discardErr := conn.Raw(func(any) error { return driver.ErrBadConn })
			if discardErr != nil && !errors.Is(discardErr, driver.ErrBadConn) {
				result = errors.Join(result, discardErr)
			}
		}
		result = errors.Join(result, conn.Close())
	}()

	// Serialize history checks, DDL and history writes across replica init containers.
	// A pinned session lock allows concurrent index DDL outside a transaction.
	g2 = g2.Session(&gorm.Session{NewDB: true, Context: g2.Statement.Context})
	g2.Statement.ConnPool = conn
	// A blocking lock query holds a snapshot that concurrent expression-index
	// builds must wait for. Poll without keeping a query open to avoid a deadlock.
	if err := wait.PollUntilContextCancel(g2.Statement.Context, time.Second, true, func(ctx context.Context) (bool, error) {
		var acquired bool
		err := g2.WithContext(ctx).Raw("SELECT pg_try_advisory_lock(hashtext('maestro'), hashtext('migrations'))").Scan(&acquired).Error
		return acquired, err
	}); err != nil {
		return err
	}

	if err := migrations.CleanUpDirtyData(g2); err != nil {
		return err
	}

	m := newGormigrate(g2)

	if err := m.Migrate(); err != nil {
		return err
	}
	return nil
}

// MigrateTo a specific migration will not seed the database, seeds are up to date with the latest
// schema based on the most recent migration
// This should be for testing purposes mainly
func MigrateTo(sessionFactory SessionFactory, migrationID string) {
	g2 := sessionFactory.New(context.Background())
	m := newGormigrate(g2)

	if err := m.MigrateTo(migrationID); err != nil {
		klog.Fatalf("Could not migrate: %v", err)
	}
}

func newGormigrate(g2 *gorm.DB) *gormigrate.Gormigrate {
	return gormigrate.New(g2, gormigrate.DefaultOptions, migrations.MigrationList)
}
