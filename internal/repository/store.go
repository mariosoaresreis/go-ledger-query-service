package repository

import (
	"context"
	"fmt"

	"github.com/jmoiron/sqlx"
)

// Store wraps *sqlx.DB and implements Transactor.
type Store struct{ db *sqlx.DB }

// NewStore creates a Store from an open *sqlx.DB.
func NewStore(db *sqlx.DB) *Store { return &Store{db: db} }

// RunInTx executes fn inside a single serialisable database transaction.
// The balance and transaction repository instances passed to fn are bound to
// that transaction; they share the same connection, so all writes are atomic.
// If fn returns an error the transaction is rolled back; otherwise it is committed.
func (s *Store) RunInTx(
	ctx context.Context,
	fn func(ctx context.Context, balRepo BalanceRepository, txRepo TransactionRepository) error,
) error {
	tx, err := s.db.BeginTxx(ctx, nil)
	if err != nil {
		return fmt.Errorf("store: begin tx: %w", err)
	}

	if err := fn(ctx, newBalanceTx(tx), newTransactionTx(tx)); err != nil {
		_ = tx.Rollback()
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("store: commit tx: %w", err)
	}
	return nil
}

