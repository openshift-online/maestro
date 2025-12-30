package db

import (
	"context"
	"errors"

	"k8s.io/klog/v2"

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
	logger := klog.FromContext(ctx)
	tx, ok := dbContext.Transaction(ctx)
	if !ok {
		logger.Error(errors.New("missing transaction"), "Could not retrieve transaction from context")
		return
	}

	if tx.MarkedForRollback() {
		if err := tx.Rollback(); err != nil {
			logger.Error(err, "Could not rollback transaction")
			return
		}
		logger.Info("Rolled back transaction")
	} else {
		if err := tx.Commit(); err != nil {
			// TODO:  what does the user see when this occurs? seems like they will get a false positive
			logger.Error(err, "Could not commit transaction")
			return
		}
	}
}

// MarkForRollback flags the transaction stored in the context for rollback and logs whatever error caused the rollback
func MarkForRollback(ctx context.Context, err error) {
	logger := klog.FromContext(ctx)
	transaction, ok := dbContext.Transaction(ctx)
	if !ok {
		logger.Error(errors.New("could not retrieve transaction from context"), "Failed to mark transaction for rollback")
		return
	}
	transaction.SetRollbackFlag(true)
	logger.Info("Marked transaction for rollback", "error", err)
}
