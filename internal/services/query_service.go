package services

import (
	"context"
	"fmt"

	"go-ledger-query-service/internal/domain"
	"go-ledger-query-service/internal/repository"
)

// QueryService handles all read-side queries.
type QueryService interface {
	GetBalance(ctx context.Context, accountID string) (*domain.AccountBalance, error)
	ListTransactions(ctx context.Context, filter repository.TransactionFilter) ([]*domain.Transaction, int, error)
	GetStatement(ctx context.Context, accountID, month string) (*domain.Statement, error)
	ListAccountsByOwner(ctx context.Context, ownerID string) ([]*domain.AccountSummary, error)
}

type queryService struct {
	balanceRepo     repository.BalanceRepository
	transactionRepo repository.TransactionRepository
}

// NewQueryService creates a new QueryService.
func NewQueryService(
	balanceRepo repository.BalanceRepository,
	transactionRepo repository.TransactionRepository,
) QueryService {
	return &queryService{
		balanceRepo:     balanceRepo,
		transactionRepo: transactionRepo,
	}
}

func (s *queryService) GetBalance(ctx context.Context, accountID string) (*domain.AccountBalance, error) {
	return s.balanceRepo.GetBalance(ctx, accountID)
}

func (s *queryService) ListTransactions(ctx context.Context, filter repository.TransactionFilter) ([]*domain.Transaction, int, error) {
	return s.transactionRepo.ListTransactions(ctx, filter)
}

func (s *queryService) GetStatement(ctx context.Context, accountID, month string) (*domain.Statement, error) {
	txns, err := s.transactionRepo.GetMonthlyTransactions(ctx, accountID, month)
	if err != nil {
		return nil, fmt.Errorf("get statement: %w", err)
	}

	// Opening balance = net of all transactions strictly before this month.
	opening, err := s.transactionRepo.SumTransactionsBefore(ctx, accountID, month)
	if err != nil {
		return nil, fmt.Errorf("get statement: opening balance: %w", err)
	}

	closing := opening
	for _, t := range txns {
		if t.Direction == "CREDIT" {
			closing += t.Amount
		} else {
			closing -= t.Amount
		}
	}

	return &domain.Statement{
		AccountID:      accountID,
		Month:          month,
		OpeningBalance: opening,
		ClosingBalance: closing,
		Entries:        flattenTxns(txns),
	}, nil
}

func (s *queryService) ListAccountsByOwner(ctx context.Context, ownerID string) ([]*domain.AccountSummary, error) {
	return s.balanceRepo.ListByOwner(ctx, ownerID)
}

func flattenTxns(ptrs []*domain.Transaction) []domain.Transaction {
	out := make([]domain.Transaction, 0, len(ptrs))
	for _, p := range ptrs {
		out = append(out, *p)
	}
	return out
}
