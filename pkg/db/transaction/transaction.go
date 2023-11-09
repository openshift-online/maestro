package transaction

import (
	"database/sql"
	"errors"
)

// By default do no roll back transaction.
// only perform rollback if explicitly set by g2.g2.MarkForRollback(ctx, err)
const defaultRollbackPolicy = false

// Transaction represents an sql transaction
type Transaction struct {
	rollbackFlag bool
	tx           *sql.Tx
	txid         int64
}

// Build Creates a new transaction object
func Build(tx *sql.Tx, id int64, rollbackFlag bool) *Transaction {
	return &Transaction{
		tx:           tx,
		txid:         id,
		rollbackFlag: defaultRollbackPolicy,
	}
}

// MarkedForRollback returns true if a transaction is flagged for rollback and false otherwise.
func (tx *Transaction) MarkedForRollback() bool {
	return tx.rollbackFlag
}

func (tx *Transaction) Tx() *sql.Tx {
	return tx.tx
}

func (tx *Transaction) TxID() int64 {
	return tx.txid
}

func (tx *Transaction) Commit() error {
	// tx must exits
	if tx.tx == nil {
		return errors.New("db: transaction hasn't been started yet")
	}

	// must call commit on 'g2' which is Gorm
	// do *not* call commit on the underlying transaction itself. Gorm does that.
	err := tx.tx.Commit()
	tx.tx = nil
	return err
}

// rollback ends the transaction by rolling back
func (tx *Transaction) Rollback() error {
	// tx must exist
	if tx.tx == nil {
		return errors.New("db: transaction hasn't been started yet")
	}
	err := tx.tx.Rollback()
	tx.tx = nil
	return err
}

func (tx *Transaction) SetRollbackFlag(flag bool) {
	tx.rollbackFlag = flag
}
