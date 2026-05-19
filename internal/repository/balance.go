package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"go-ledger-query-service/internal/domain"

	"github.com/jmoiron/sqlx"
)

// querier is satisfied by both *sqlx.DB and *sqlx.Tx so repository
// implementations work inside or outside a database transaction.
type querier interface {
	GetContext(ctx context.Context, dest any, query string, args ...any) error
	SelectContext(ctx context.Context, dest any, query string, args ...any) error
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

type balanceDB struct{ q querier }

// NewBalanceRepository creates a PostgreSQL-backed balance repository.
func NewBalanceRepository(db *sqlx.DB) BalanceRepository { return &balanceDB{q: db} }

// newBalanceTx binds a balance repository to an existing transaction.
func newBalanceTx(tx *sqlx.Tx) BalanceRepository { return &balanceDB{q: tx} }

func (r *balanceDB) GetBalance(ctx context.Context, accountID string) (*domain.AccountBalance, error) {
	bal := &domain.AccountBalance{}
	err := r.q.GetContext(ctx, bal, `
		SELECT account_id, owner_id, currency, balance, status, as_of
		FROM account_balances
		WHERE account_id = $1
	`, accountID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("account %s not found", accountID)
		}
		return nil, fmt.Errorf("balance repo: get: %w", err)
	}
	return bal, nil
}

func (r *balanceDB) UpsertBalance(ctx context.Context, balance *domain.AccountBalance) error {
	_, err := r.q.ExecContext(ctx, `
		INSERT INTO account_balances (account_id, owner_id, currency, balance, status, as_of)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (account_id) DO UPDATE
		SET owner_id = EXCLUDED.owner_id,
		    currency = EXCLUDED.currency,
		    balance = EXCLUDED.balance,
		    status = EXCLUDED.status,
		    as_of = EXCLUDED.as_of
	`, balance.AccountID, balance.OwnerID, balance.Currency, balance.Balance, balance.Status, balance.AsOf)
	if err != nil {
		return fmt.Errorf("balance repo: upsert: %w", err)
	}
	return nil
}

func (r *balanceDB) ListByOwner(ctx context.Context, ownerID string) ([]*domain.AccountSummary, error) {
	var summaries []*domain.AccountSummary
	err := r.q.SelectContext(ctx, &summaries, `
		SELECT account_id, owner_id, currency, balance, status
		FROM account_balances
		WHERE owner_id = $1
	`, ownerID)
	if err != nil {
		return nil, fmt.Errorf("balance repo: list by owner: %w", err)
	}
	return summaries, nil
}
