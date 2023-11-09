// Package db_context dbContext provides a wrapper around db context handling to allow access to the db context without
// requiring importing the db package, thus avoiding cyclic imports
package db_context

import (
	"context"

	"github.com/openshift-online/maestro/pkg/db/transaction"
)

type contextKey int

const (
	transactionKey contextKey = iota
)

// WithTransaction adds the transaction to the context and returns a new context
func WithTransaction(ctx context.Context, tx *transaction.Transaction) context.Context {
	return context.WithValue(ctx, transactionKey, tx)
}

// Transaction extracts the transaction value from the context
func Transaction(ctx context.Context) (tx *transaction.Transaction, ok bool) {
	tx, ok = ctx.Value(transactionKey).(*transaction.Transaction)
	return tx, ok
}

// Return the transaction ID from the context, if it exists. If there is no transaction, ok is false.
func TxID(ctx context.Context) (id int64, ok bool) {
	tx, ok := Transaction(ctx)
	if !ok {
		return 0, false
	}
	return tx.TxID(), true
}
