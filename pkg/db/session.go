package db

import (
	"context"
	"database/sql"

	"github.com/lib/pq"
	"gorm.io/gorm"

	"github.com/openshift-online/maestro/pkg/config"
)

type SessionFactory interface {
	Init(*config.DatabaseConfig)
	DirectDB() *sql.DB
	New(ctx context.Context) *gorm.DB
	CheckConnection() error
	Close() error
	ResetDB()
	NewListener(ctx context.Context, channel string, callback func(id string)) *pq.Listener
}
