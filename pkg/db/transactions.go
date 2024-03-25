package db

import (
	"github.com/openshift-online/maestro/pkg/db/transaction"
)

// By default do no roll back transaction.
// only perform rollback if explicitly set by g2.g2.MarkForRollback(ctx, err)
const defaultRollbackPolicy = false

// newTransaction constructs a new Transaction object.
func newTransaction(connection SessionFactory) (*transaction.Transaction, error) {
	if connection == nil {
		// This happens in non-integration tests
		return nil, nil
	}

	dbx := connection.DirectDB()
	tx, err := dbx.Begin()
	if err != nil {
		return nil, err
	}

	// current transaction ID set by postgres.  these are *not* distinct across time
	// and do get reset after postgres performs "vacuuming" to reclaim used IDs.
	var txid int64
	row := tx.QueryRow("select txid_current()")
	if row != nil {
		err := row.Scan(&txid)
		if err != nil {
			return nil, err
		}
	}

	return transaction.Build(tx, txid, defaultRollbackPolicy), nil
}
