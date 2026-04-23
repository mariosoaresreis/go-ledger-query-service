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
