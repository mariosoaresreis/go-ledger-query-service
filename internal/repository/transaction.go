package repository

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
	"go-ledger-query-service/internal/domain"
)

type transactionDB struct {
	db *bun.DB
}

// NewTransactionRepository creates a PostgreSQL-backed transaction repository.
func NewTransactionRepository(db *bun.DB) TransactionRepository {
	return &transactionDB{db: db}
}

func (r *transactionDB) ListTransactions(ctx context.Context, filter TransactionFilter) ([]*domain.Transaction, int, error) {
	var txns []*domain.Transaction

	q := r.db.NewSelect().Model(&txns).Where("account_id = ?", filter.AccountID)

	if filter.From != "" {
		q = q.Where("created_at >= ?", filter.From)
	}
	if filter.To != "" {
		q = q.Where("created_at <= ?", filter.To+" 23:59:59")
	}
	if filter.Direction != "" {
		q = q.Where("direction = ?", filter.Direction)
	}

	total, err := q.ScanAndCount(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("transaction repo: list: %w", err)
	}

	// Pagination (applied after count)
	page := filter.Page
	if page < 0 {
		page = 0
	}
	size := filter.Size
	if size <= 0 {
		size = 20
	}

	var paged []*domain.Transaction
	err = r.db.NewSelect().Model(&paged).
		Where("account_id = ?", filter.AccountID).
		OrderExpr("created_at DESC").
		Limit(size).
		Offset(page * size).
		Scan(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("transaction repo: list paged: %w", err)
	}

	return paged, total, nil
}

func (r *transactionDB) InsertTransaction(ctx context.Context, tx *domain.Transaction) error {
	_, err := r.db.NewInsert().Model(tx).On("CONFLICT (id) DO NOTHING").Exec(ctx)
	if err != nil {
		return fmt.Errorf("transaction repo: insert: %w", err)
	}
	return nil
}

func (r *transactionDB) GetMonthlyTransactions(ctx context.Context, accountID, month string) ([]*domain.Transaction, error) {
	// month format: YYYY-MM
	start := month + "-01"
	// end of month handled by < next month
	var txns []*domain.Transaction
	err := r.db.NewSelect().Model(&txns).
		Where("account_id = ?", accountID).
		Where("TO_CHAR(created_at, 'YYYY-MM') = ?", month).
		OrderExpr("created_at ASC").
		Scan(ctx)
	_ = start
	if err != nil {
		return nil, fmt.Errorf("transaction repo: monthly: %w", err)
	}
	return txns, nil
}
