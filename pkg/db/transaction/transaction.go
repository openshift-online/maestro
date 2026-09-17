package transaction

import (
	"errors"

	"gorm.io/gorm"
)

// By default do no roll back transaction.
// only perform rollback if explicitly set by g2.g2.MarkForRollback(ctx, err)
const defaultRollbackPolicy = false

// Transaction represents an sql transaction
type Transaction struct {
	rollbackFlag bool
	tx           *gorm.DB
	txid         int64
}

// Build Creates a new transaction object
func Build(tx *gorm.DB, id int64, rollbackFlag bool) *Transaction {
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
	db := tx.tx.Commit()
	tx.tx = nil
	return db.Error
}

// rollback ends the transaction by rolling back
func (tx *Transaction) Rollback() error {
	// tx must exist
	if tx.tx == nil {
		return errors.New("db: transaction hasn't been started yet")
	}
	db := tx.tx.Rollback()
	tx.tx = nil
	return db.Error
}

func (tx *Transaction) SetRollbackFlag(flag bool) {
	tx.rollbackFlag = flag
}

func (tx *Transaction) Session(config *gorm.Session) *gorm.DB {
	return tx.tx.Session(config)
}
