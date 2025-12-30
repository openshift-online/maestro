package db_session

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/lib/pq"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
	"k8s.io/klog/v2"

	"github.com/openshift-online/maestro/pkg/config"
	"github.com/openshift-online/maestro/pkg/db"
)

var testOnce sync.Once

type Test struct {
	config *config.DatabaseConfig
	g2     *gorm.DB
	// Direct database connection.
	// It is used:
	// - to setup/close connection because GORM V2 removed gorm.Close()
	// - to work with pq.CopyIn because connection returned by GORM V2 gorm.DB() in "not the same"
	db *sql.DB

	wasDisconnected bool
}

var _ db.SessionFactory = &Test{}

func NewTestFactory(config *config.DatabaseConfig) *Test {
	conn := &Test{}
	conn.Init(config)
	return conn
}

// The approach:
// Every new Postgres database is implicitly copied from a template database `template1`. Any changes to template1
// are then copied to a new database, and this copy is a cheap filesystem operation.

// Init will:
// - initialize a template1 DB with migrations
// - rebuild Maestro DB from template1
// - return a new connection factory
// Go includes database connection pooling in the platform. Gorm uses the same and provides a method to
// clone a connection via New(), which is safe for use by concurrent Goroutines.
func (f *Test) Init(config *config.DatabaseConfig) {
	// Only the first time
	testOnce.Do(func() {
		if err := initDatabase(config, db.Migrate); err != nil {
			klog.Error(err, "error initializing test database")
			return
		}

		if err := resetDB(config); err != nil {
			klog.Error(err, "error resetting test database")
			return
		}
	})

	f.config = config
	f.db, f.g2 = connectFactory(config)
}

func initDatabase(config *config.DatabaseConfig, migrate func(db2 *gorm.DB) error) error {
	// - Connect to `template1` DB
	dbx, g2, cleanup := connect("template1", config)
	defer cleanup()

	for _, err := dbx.Exec(`select 1`); err != nil; {
		time.Sleep(100 * time.Millisecond)
	}

	// - Run migrations
	return migrate(g2)
}

func resetDB(config *config.DatabaseConfig) error {
	// Reconnect to the default `postgres` database, so we can drop the existing db and recreate it
	dbx, _, cleanup := connect("postgres", config)
	defer cleanup()

	// Drop `all` connections to both `template1` and Maestro DB, so it can be dropped and created
	if err := dropConnections(dbx, "template1"); err != nil {
		return err
	}
	if err := dropConnections(dbx, config.Name); err != nil {
		return err
	}

	// Rebuild Maestro DB
	query := fmt.Sprintf("DROP DATABASE IF EXISTS %s", config.Name)
	if _, err := dbx.Exec(query); err != nil {
		return fmt.Errorf("SQL failed to DROP database %s: %s", config.Name, err.Error())
	}
	query = fmt.Sprintf("CREATE DATABASE %s TEMPLATE template1", config.Name)
	if _, err := dbx.Exec(query); err != nil {
		return fmt.Errorf("SQL failed to CREATE database %s: %s", config.Name, err.Error())
	}
	// As `template1` had all migrations, so now Maestro DB has them too!
	return nil
}

// connect to database specified by `name` and return connections + cleanup function
func connect(name string, config *config.DatabaseConfig) (*sql.DB, *gorm.DB, func()) {
	var (
		dbx *sql.DB
		g2  *gorm.DB
		err error
	)

	connConfig, err := pgx.ParseConfig(config.ConnectionStringWithName(name, config.SSLMode != disable))
	if err != nil {
		panic(fmt.Sprintf(
			"GORM failed to parse the connection string: %s\nError: %s",
			config.LogSafeConnectionStringWithName(name, config.SSLMode != disable),
			err.Error(),
		))
	}

	dbx = stdlib.OpenDB(*connConfig, stdlib.OptionBeforeConnect(setPassword(config)))

	// Connect GORM to use the same connection
	conf := &gorm.Config{
		PrepareStmt:            false,
		FullSaveAssociations:   false,
		SkipDefaultTransaction: true,
		Logger:                 gormlogger.Default.LogMode(gormlogger.Silent),
	}
	g2, err = gorm.Open(postgres.New(postgres.Config{
		Conn: dbx,
		// Disable implicit prepared statement usage (GORM V2 uses pgx as database/sql driver and it enables prepared
		// statement cache by default)
		// In migrations we both change tables' structure and running SQLs to modify data.
		// This way all prepared statements becomes invalid.
		PreferSimpleProtocol: true,
	}), conf)
	if err != nil {
		panic(fmt.Sprintf(
			"GORM failed to connect to %s database %s with connection string: %s\nError: %s",
			config.Dialect,
			config.Name,
			config.LogSafeConnectionString(config.SSLMode != disable),
			err.Error(),
		))
	}

	return dbx, g2, func() {
		if err := dbx.Close(); err != nil {
			panic(err)
		}
	}
}

// KILL all connections to the specified DB
func dropConnections(dbx *sql.DB, name string) error {
	query := `
		select pg_terminate_backend(pg_stat_activity.pid)
		from pg_stat_activity
		where pg_stat_activity.datname = $1 and pid <> pg_backend_pid()`
	_, err := dbx.Exec(query, name)
	if err != nil {
		return err
	}
	return nil
}

func connectFactory(config *config.DatabaseConfig) (*sql.DB, *gorm.DB) {
	var (
		dbx *sql.DB
		g2  *gorm.DB
	)
	dbx, g2, _ = connect(config.Name, config)
	dbx.SetMaxOpenConns(config.MaxOpenConnections)

	return dbx, g2
}

func (f *Test) DirectDB() *sql.DB {
	return f.db
}

func (f *Test) New(ctx context.Context) *gorm.DB {
	if f.wasDisconnected {
		// Connection was killed in order to reset DB
		f.db, f.g2 = connectFactory(f.config)
		f.wasDisconnected = false
	}

	conn := f.g2.Session(&gorm.Session{
		Context: ctx,
		Logger:  f.g2.Logger.LogMode(gormlogger.Silent),
	})
	if f.config.Debug {
		conn = conn.Debug()
	}
	return conn
}

// CheckConnection checks to ensure a connection is present
func (f *Test) CheckConnection() error {
	_, err := f.db.Exec("SELECT 1")
	return err
}

func (f *Test) Close() error {
	return f.db.Close()
}

func (f *Test) ResetDB() {
	resetDB(f.config)
	f.wasDisconnected = true
}

func (f *Test) NewListener(ctx context.Context, channel string, callback func(id string)) *pq.Listener {
	listener := newListener(ctx, f.config, channel)
	go waitForNotification(ctx, listener, f.config, channel, callback)
	return listener
}
