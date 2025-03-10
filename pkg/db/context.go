package db

import (
	"context"

	dbContext "github.com/openshift-online/maestro/pkg/db/db_context"
)

// NewContext returns a new context with transaction stored in it.
// Upon error, the original context is still returned along with an error
func NewContext(ctx context.Context, connection SessionFactory) (context.Context, error) {
	tx, err := newTransaction(connection)
	if err != nil {
		return ctx, err
	}

	ctx = dbContext.WithTransaction(ctx, tx)

	return ctx, nil
}

// Resolve resolves the current transaction according to the rollback flag.
func Resolve(ctx context.Context) {
	tx, ok := dbContext.Transaction(ctx)
	if !ok {
		log.Error("Could not retrieve transaction from context")
		return
	}

	if tx.MarkedForRollback() {
		if err := tx.Rollback(); err != nil {
			log.With("error", err.Error()).Error("Could not rollback transaction")
			return
		}
		log.Infof("Rolled back transaction")
	} else {
		if err := tx.Commit(); err != nil {
			// TODO:  what does the user see when this occurs? seems like they will get a false positive
			log.With("error", err.Error()).Error("Could not commit transaction")
			return
		}
	}
}

// MarkForRollback flags the transaction stored in the context for rollback and logs whatever error caused the rollback
func MarkForRollback(ctx context.Context, err error) {
	transaction, ok := dbContext.Transaction(ctx)
	if !ok {
		log.Error("failed to mark transaction for rollback: could not retrieve transaction from context")
		return
	}
	transaction.SetRollbackFlag(true)
	log.Infof("Marked transaction for rollback, err: %v", err)
}
