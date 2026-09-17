package db

import (
	"context"
	"fmt"

	"github.com/openshift-online/maestro/pkg/db/transaction"
)

// By default do no roll back transaction.
// only perform rollback if explicitly set by g2.g2.MarkForRollback(ctx, err)
const defaultRollbackPolicy = false

// newTransaction constructs a new Transaction object.
func newTransaction(ctx context.Context, connection SessionFactory) (*transaction.Transaction, error) {
	if connection == nil {
		// This happens in non-integration tests
		return nil, nil
	}

	db := connection.New(ctx)
	tx := db.Begin()
	if tx.Error != nil {
		return nil, tx.Error
	}

	// current transaction ID set by postgres.  these are *not* distinct across time
	// and do get reset after postgres performs "vacuuming" to reclaim used IDs.
	var txid int64
	row := tx.Raw("select txid_current()")
	if row != nil {
		err := row.Scan(&txid).Error
		if err != nil {
			// rollback the transaction we just started
			rollbackErr := tx.Rollback().Error
			if rollbackErr != nil {
				return nil, fmt.Errorf("could not rollback transaction after error retrieving txid: '%s' '%s'", err, rollbackErr)
			}
			return nil, err
		}
	}

	return transaction.Build(tx, txid, defaultRollbackPolicy), nil
}
