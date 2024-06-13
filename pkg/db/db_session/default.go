package db_session

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/lib/pq"

	"github.com/openshift-online/maestro/pkg/config"
	"github.com/openshift-online/maestro/pkg/db"
	ocmlogger "github.com/openshift-online/maestro/pkg/logger"
)

type Default struct {
	config *config.DatabaseConfig

	g2 *gorm.DB
	// Direct database connection.
	// It is used:
	// - to setup/close connection because GORM V2 removed gorm.Close()
	// - to work with pq.CopyIn because connection returned by GORM V2 gorm.DB() in "not the same"
	db *sql.DB
}

var _ db.SessionFactory = &Default{}

func NewProdFactory(config *config.DatabaseConfig) *Default {
	conn := &Default{}
	conn.Init(config)
	return conn
}

// Init will initialize a singleton connection as needed and return the same instance.
// Go includes database connection pooling in the platform. Gorm uses the same and provides a method to
// clone a connection via New(), which is safe for use by concurrent Goroutines.
func (f *Default) Init(config *config.DatabaseConfig) {
	// Only the first time
	once.Do(func() {
		var (
			dbx *sql.DB
			g2  *gorm.DB
			err error
		)

		// Open connection to DB via standard library
		dbx, err = sql.Open(config.Dialect, config.ConnectionString(config.SSLMode != disable))
		if err != nil {
			dbx, err = sql.Open(config.Dialect, config.ConnectionString(false))
			if err != nil {
				panic(fmt.Sprintf(
					"SQL failed to connect to %s database %s with connection string: %s\nError: %s",
					config.Dialect,
					config.Name,
					config.LogSafeConnectionString(config.SSLMode != disable),
					err.Error(),
				))
			}
		}
		dbx.SetMaxOpenConns(config.MaxOpenConnections)

		// Connect GORM to use the same connection
		conf := &gorm.Config{
			PrepareStmt:          false,
			FullSaveAssociations: false,
		}
		g2, err = gorm.Open(postgres.New(postgres.Config{
			Conn: dbx,
			// Disable implicit prepared statement usage (GORM V2 uses pgx as database/sql driver and it enables prepared
			/// statement cache by default)
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

		f.config = config
		f.g2 = g2
		f.db = dbx
	})
}

func (f *Default) DirectDB() *sql.DB {
	return f.db
}

func waitForNotification(ctx context.Context, l *pq.Listener, callback func(id string)) {
	logger := ocmlogger.NewOCMLogger(ctx)
	for {
		select {
		case <-ctx.Done():
			logger.Infof("Context cancelled, stopping channel monitor")
			return
		case n := <-l.Notify:
			if n != nil {
				logger.V(4).Infof("Received event from channel [%s] : %s", n.Channel, n.Extra)
				callback(n.Extra)
			}
		case <-time.After(10 * time.Second):
			logger.V(10).Infof("Received no events on channel during interval. Pinging source")
			go func() {
				l.Ping()
			}()
		}
	}
}

func newListener(ctx context.Context, connstr, channel string, callback func(id string)) {
	logger := ocmlogger.NewOCMLogger(ctx)

	plog := func(ev pq.ListenerEventType, err error) {
		if err != nil {
			logger.Error(err.Error())
		}
	}
	listener := pq.NewListener(connstr, 10*time.Second, time.Minute, plog)
	err := listener.Listen(channel)
	if err != nil {
		panic(err)
	}

	logger.Infof("Starting channeling monitor for %s", channel)
	waitForNotification(ctx, listener, callback)
}

func (f *Default) NewListener(ctx context.Context, channel string, callback func(id string)) {
	newListener(ctx, f.config.ConnectionString(true), channel, callback)
}

func (f *Default) New(ctx context.Context) *gorm.DB {
	conn := f.g2.Session(&gorm.Session{
		Context: ctx,
		Logger:  f.g2.Logger.LogMode(logger.Silent),
	})
	if f.config.Debug {
		conn = conn.Debug()
	}
	return conn
}

func (f *Default) CheckConnection() error {
	return f.g2.Exec("SELECT 1").Error
}

// Close will close the connection to the database.
// THIS MUST **NOT** BE CALLED UNTIL THE SERVER/PROCESS IS EXITING!!
// This should only ever be called once for the entire duration of the application and only at the end.
func (f *Default) Close() error {
	return f.db.Close()
}

func (f *Default) ResetDB() {
	panic("ResetDB is not implemented for non-integration-test env")
}
