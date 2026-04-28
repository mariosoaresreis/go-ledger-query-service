package repository

import (
	"context"
	"fmt"
	"strings"

	"github.com/jmoiron/sqlx"
	"go-ledger-query-service/internal/domain"
)

type transactionDB struct {
	db *sqlx.DB
}

// NewTransactionRepository creates a PostgreSQL-backed transaction repository.
func NewTransactionRepository(db *sqlx.DB) TransactionRepository {
	return &transactionDB{db: db}
}

func (r *transactionDB) ListTransactions(ctx context.Context, filter TransactionFilter) ([]*domain.Transaction, int, error) {
	where := []string{"account_id = $1"}
	args := []any{filter.AccountID}
	argPos := 2

	if filter.From != "" {
		where = append(where, fmt.Sprintf("created_at >= $%d", argPos))
		args = append(args, filter.From)
		argPos++
	}
	if filter.To != "" {
		where = append(where, fmt.Sprintf("created_at <= $%d", argPos))
		args = append(args, filter.To+" 23:59:59")
		argPos++
	}
	if filter.Direction != "" {
		where = append(where, fmt.Sprintf("direction = $%d", argPos))
		args = append(args, filter.Direction)
		argPos++
	}
	whereClause := strings.Join(where, " AND ")

	var total int
	err := r.db.GetContext(ctx, &total,
		"SELECT COUNT(1) FROM transactions WHERE "+whereClause,
		args...,
	)
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

	listArgs := append([]any{}, args...)
	listArgs = append(listArgs, size, page*size)

	var paged []*domain.Transaction
	err = r.db.SelectContext(ctx, &paged,
		"SELECT id, account_id, event_type, amount, currency, direction, reference, created_at "+
			"FROM transactions WHERE "+whereClause+" ORDER BY created_at DESC LIMIT $"+fmt.Sprintf("%d", argPos)+" OFFSET $"+fmt.Sprintf("%d", argPos+1),
		listArgs...,
	)
	if err != nil {
		return nil, 0, fmt.Errorf("transaction repo: list paged: %w", err)
	}

	return paged, total, nil
}

func (r *transactionDB) InsertTransaction(ctx context.Context, tx *domain.Transaction) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO transactions (id, account_id, event_type, amount, currency, direction, reference, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		ON CONFLICT (id) DO NOTHING
	`, tx.ID, tx.AccountID, tx.EventType, tx.Amount, tx.Currency, tx.Direction, tx.Reference, tx.CreatedAt)
	if err != nil {
		return fmt.Errorf("transaction repo: insert: %w", err)
	}
	return nil
}

func (r *transactionDB) GetMonthlyTransactions(ctx context.Context, accountID, month string) ([]*domain.Transaction, error) {
	var txns []*domain.Transaction
	err := r.db.SelectContext(ctx, &txns, `
		SELECT id, account_id, event_type, amount, currency, direction, reference, created_at
		FROM transactions
		WHERE account_id = $1
		  AND TO_CHAR(created_at, 'YYYY-MM') = $2
		ORDER BY created_at ASC
	`, accountID, month)
	if err != nil {
		return nil, fmt.Errorf("transaction repo: monthly: %w", err)
	}
	return txns, nil
}
