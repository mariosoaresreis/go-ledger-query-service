package repository

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"
	"go-ledger-query-service/internal/domain"
)

type transactionDB struct{ q querier }

// NewTransactionRepository creates a PostgreSQL-backed transaction repository.
func NewTransactionRepository(db *sqlx.DB) TransactionRepository { return &transactionDB{q: db} }

// newTransactionTx binds a transaction repository to an existing DB transaction.
func newTransactionTx(tx *sqlx.Tx) TransactionRepository { return &transactionDB{q: tx} }

func (r *transactionDB) ListTransactions(ctx context.Context, filter TransactionFilter) ([]*domain.Transaction, int, error) {
	where := []string{"account_id = $1"}
	args := []any{filter.AccountID}
	argPos := 2

	if filter.From != "" {
		from, err := time.Parse("2006-01-02", filter.From)
		if err != nil {
			return nil, 0, fmt.Errorf("transaction repo: invalid from date %q: %w", filter.From, err)
		}
		where = append(where, fmt.Sprintf("created_at >= $%d", argPos))
		args = append(args, from.UTC())
		argPos++
	}
	if filter.To != "" {
		to, err := time.Parse("2006-01-02", filter.To)
		if err != nil {
			return nil, 0, fmt.Errorf("transaction repo: invalid to date %q: %w", filter.To, err)
		}
		// Use exclusive upper-bound (start of next day) so the full To-date is included.
		where = append(where, fmt.Sprintf("created_at < $%d", argPos))
		args = append(args, to.AddDate(0, 0, 1).UTC())
		argPos++
	}
	if filter.Direction != "" {
		where = append(where, fmt.Sprintf("direction = $%d", argPos))
		args = append(args, filter.Direction)
		argPos++
	}
	whereClause := strings.Join(where, " AND ")

	var total int
	if err := r.q.GetContext(ctx, &total,
		"SELECT COUNT(1) FROM transactions WHERE "+whereClause, args...,
	); err != nil {
		return nil, 0, fmt.Errorf("transaction repo: count: %w", err)
	}

	page := filter.Page
	if page < 0 {
		page = 0
	}
	size := filter.Size
	if size <= 0 {
		size = 20
	}

	listArgs := append(append([]any{}, args...), size, page*size)
	limitPos := argPos
	offsetPos := argPos + 1

	var paged []*domain.Transaction
	if err := r.q.SelectContext(ctx, &paged,
		"SELECT id, account_id, event_type, amount, currency, direction, reference, created_at "+
			"FROM transactions WHERE "+whereClause+
			fmt.Sprintf(" ORDER BY created_at DESC LIMIT $%d OFFSET $%d", limitPos, offsetPos),
		listArgs...,
	); err != nil {
		return nil, 0, fmt.Errorf("transaction repo: list paged: %w", err)
	}
	return paged, total, nil
}

func (r *transactionDB) InsertTransaction(ctx context.Context, tx *domain.Transaction) error {
	_, err := r.q.ExecContext(ctx, `
		INSERT INTO transactions (id, account_id, event_type, amount, currency, direction, reference, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		ON CONFLICT (id) DO NOTHING
	`, tx.ID, tx.AccountID, tx.EventType, tx.Amount, tx.Currency, tx.Direction, tx.Reference, tx.CreatedAt)
	if err != nil {
		return fmt.Errorf("transaction repo: insert: %w", err)
	}
	return nil
}

// GetMonthlyTransactions returns all transactions for an account within the given month (YYYY-MM).
// Uses an index-friendly date range instead of TO_CHAR so the created_at index is utilised.
func (r *transactionDB) GetMonthlyTransactions(ctx context.Context, accountID, month string) ([]*domain.Transaction, error) {
	start, err := time.Parse("2006-01", month)
	if err != nil {
		return nil, fmt.Errorf("transaction repo: invalid month %q: %w", month, err)
	}
	end := start.AddDate(0, 1, 0)

	var txns []*domain.Transaction
	err = r.q.SelectContext(ctx, &txns, `
		SELECT id, account_id, event_type, amount, currency, direction, reference, created_at
		FROM transactions
		WHERE account_id = $1
		  AND created_at >= $2
		  AND created_at < $3
		ORDER BY created_at ASC
	`, accountID, start.UTC(), end.UTC())
	if err != nil {
		return nil, fmt.Errorf("transaction repo: monthly: %w", err)
	}
	return txns, nil
}

// SumTransactionsBefore computes the net balance (credits − debits) for all transactions
// strictly before `before` (YYYY-MM-DD or YYYY-MM format).  Used for statement opening balances.
func (r *transactionDB) SumTransactionsBefore(ctx context.Context, accountID string, before string) (int64, error) {
	// Accept both "YYYY-MM" and "YYYY-MM-DD".
	var boundary time.Time
	var err error
	if len(before) == 7 {
		boundary, err = time.Parse("2006-01", before)
	} else {
		boundary, err = time.Parse("2006-01-02", before)
	}
	if err != nil {
		return 0, fmt.Errorf("transaction repo: sum before: invalid boundary %q: %w", before, err)
	}

	var net int64
	err = r.q.GetContext(ctx, &net, `
		SELECT COALESCE(
			SUM(CASE WHEN direction = 'CREDIT' THEN amount ELSE -amount END),
			0
		)
		FROM transactions
		WHERE account_id = $1
		  AND created_at < $2
	`, accountID, boundary.UTC())
	if err != nil {
		return 0, fmt.Errorf("transaction repo: sum before: %w", err)
	}
	return net, nil
}

