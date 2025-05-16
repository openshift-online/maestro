package db_session

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/lib/pq"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"

	"github.com/openshift-online/maestro/pkg/config"
	"github.com/openshift-online/maestro/pkg/constants"
	"github.com/openshift-online/maestro/pkg/db"
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

		connConfig, err := pgx.ParseConfig(config.ConnectionString(config.SSLMode != disable))
		if err != nil {
			panic(fmt.Sprintf(
				"GORM failed to parse the connection string: %s\nError: %s",
				config.LogSafeConnectionString(config.SSLMode != disable),
				err.Error(),
			))
		}

		dbx = stdlib.OpenDB(*connConfig, stdlib.OptionBeforeConnect(setPassword(config)))
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

func setPassword(dbConfig *config.DatabaseConfig) func(ctx context.Context, connConfig *pgx.ConnConfig) error {
	return func(ctx context.Context, connConfig *pgx.ConnConfig) error {
		if dbConfig.AuthMethod == constants.AuthMethodPassword {
			connConfig.Password = dbConfig.Password
			return nil
		} else if dbConfig.AuthMethod == constants.AuthMethodMicrosoftEntra {
			if isExpired(dbConfig.Token) {
				token, err := getAccessToken(ctx, dbConfig)
				if err != nil {
					return err
				}
				connConfig.Password = token.Token
				dbConfig.Token = token
			} else {
				connConfig.Password = dbConfig.Token.Token
			}
		}
		return nil
	}
}

func getAccessToken(ctx context.Context, dbConfig *config.DatabaseConfig) (*azcore.AccessToken, error) {
	// ARO-HCP environment variable configuration is set by the Azure workload identity webhook.
	// Use [WorkloadIdentityCredential] directly when not using the webhook or needing more control over its configuration.
	cred, err := azidentity.NewDefaultAzureCredential(nil)
	if err != nil {
		return nil, err
	}
	token, err := cred.GetToken(ctx, policy.TokenRequestOptions{Scopes: []string{dbConfig.TokenRequestScope}})
	if err != nil {
		return nil, err
	}
	return &token, nil
}

func isExpired(accessToken *azcore.AccessToken) bool {
	return accessToken == nil ||
		time.Until(accessToken.ExpiresOn).Seconds() < constants.MinTokenLifeThreshold
}

func (f *Default) DirectDB() *sql.DB {
	return f.db
}

func waitForNotification(ctx context.Context, l *pq.Listener, dbConfig *config.DatabaseConfig, channel string, callback func(id string)) {
	for {
		select {
		case <-ctx.Done():
			log.Infof("Context cancelled, stopping channel [%s] monitor", channel)
			return
		case n := <-l.Notify:
			if n != nil {
				log.Debugf("Received event from channel [%s] : %s", n.Channel, n.Extra)
				callback(n.Extra)
			} else {
				// nil notification means the connection was closed
				log.Infof("recreate the listener for channel [%s] due to the connection loss", channel)
				l.Close()
				// recreate the listener
				l = newListener(ctx, dbConfig, channel)
			}
		case <-time.After(10 * time.Second):
			log.Debugf("Received no events on channel [%s] during interval. Pinging source", channel)
			if err := l.Ping(); err != nil {
				log.Infof("recreate the listener due to ping failed, %s", err.Error())
				l.Close()
				// recreate the listener
				l = newListener(ctx, dbConfig, channel)
			}
		}
	}
}

func newListener(ctx context.Context, dbConfig *config.DatabaseConfig, channel string) *pq.Listener {
	plog := func(ev pq.ListenerEventType, err error) {
		if err != nil {
			log.Error(fmt.Sprintf("Listener: the state of the underlying database connection changes, (eventType=%d) %v", ev, err.Error()))
		}
	}
	connstr := dbConfig.ConnectionString(true)
	// append the password to the connection string
	if dbConfig.AuthMethod == constants.AuthMethodPassword {
		connstr += fmt.Sprintf(" password='%s'", dbConfig.Password)
	} else if dbConfig.AuthMethod == constants.AuthMethodMicrosoftEntra {
		token, err := getAccessToken(ctx, dbConfig)
		if err != nil {
			panic(err)
		}
		connstr += fmt.Sprintf(" password='%s'", token.Token)
	}

	listener := pq.NewListener(connstr, 10*time.Second, time.Minute, plog)
	err := listener.Listen(channel)
	if err != nil {
		panic(err)
	}

	return listener
}

func (f *Default) NewListener(ctx context.Context, channel string, callback func(id string)) *pq.Listener {
	listener := newListener(ctx, f.config, channel)

	log.Infof("Starting channeling monitor for %s", channel)
	go waitForNotification(ctx, listener, f.config, channel, callback)
	return listener
}

func (f *Default) New(ctx context.Context) *gorm.DB {
	conn := f.g2.Session(&gorm.Session{
		Context: ctx,
		Logger:  f.g2.Logger.LogMode(gormlogger.Silent),
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
