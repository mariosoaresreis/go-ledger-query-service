package repository

import (
	"context"
	"go-ledger-query-service/internal/domain"
)

// BalanceRepository reads account balance projections.
type BalanceRepository interface {
	GetBalance(ctx context.Context, accountID string) (*domain.AccountBalance, error)
	UpsertBalance(ctx context.Context, balance *domain.AccountBalance) error
	ListByOwner(ctx context.Context, ownerID string) ([]*domain.AccountSummary, error)
}

// TransactionRepository reads and writes the transaction projection.
type TransactionRepository interface {
	ListTransactions(ctx context.Context, filter TransactionFilter) ([]*domain.Transaction, int, error)
	InsertTransaction(ctx context.Context, tx *domain.Transaction) error
	GetMonthlyTransactions(ctx context.Context, accountID, month string) ([]*domain.Transaction, error)
	// SumTransactionsBefore returns the net balance from all transactions strictly before `before`.
	// Used to compute statement opening balances without a snapshot table.
	SumTransactionsBefore(ctx context.Context, accountID string, before string) (int64, error)
}

// Transactor executes fn inside a single database transaction, rolling back on any error.
// Both repo instances passed to fn are bound to that transaction.
type Transactor interface {
	RunInTx(ctx context.Context, fn func(ctx context.Context, balRepo BalanceRepository, txRepo TransactionRepository) error) error
}

// TransactionFilter supports paginated and filtered queries.
type TransactionFilter struct {
	AccountID string
	From      string // YYYY-MM-DD
	To        string // YYYY-MM-DD
	Direction string // CREDIT | DEBIT | "" (all)
	Page      int
	Size      int
}
