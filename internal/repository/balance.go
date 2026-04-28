package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/jmoiron/sqlx"
	"go-ledger-query-service/internal/domain"
)

type balanceDB struct {
	db *sqlx.DB
}

// NewBalanceRepository creates a PostgreSQL-backed balance repository.
func NewBalanceRepository(db *sqlx.DB) BalanceRepository {
	return &balanceDB{db: db}
}

func (r *balanceDB) GetBalance(ctx context.Context, accountID string) (*domain.AccountBalance, error) {
	bal := &domain.AccountBalance{}
	err := r.db.GetContext(ctx, bal, `
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
	_, err := r.db.ExecContext(ctx, `
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
	err := r.db.SelectContext(ctx, &summaries, `
		SELECT account_id, owner_id, currency, balance, status
		FROM account_balances
		WHERE owner_id = $1
	`, ownerID)
	if err != nil {
		return nil, fmt.Errorf("balance repo: list by owner: %w", err)
	}
	return summaries, nil
}
