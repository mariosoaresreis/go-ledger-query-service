package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/uptrace/bun"
	"go-ledger-query-service/internal/domain"
)

type balanceDB struct {
	db *bun.DB
}

// NewBalanceRepository creates a PostgreSQL-backed balance repository.
func NewBalanceRepository(db *bun.DB) BalanceRepository {
	return &balanceDB{db: db}
}

func (r *balanceDB) GetBalance(ctx context.Context, accountID string) (*domain.AccountBalance, error) {
	bal := &domain.AccountBalance{}
	err := r.db.NewSelect().Model(bal).Where("account_id = ?", accountID).Scan(ctx)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("account %s not found", accountID)
		}
		return nil, fmt.Errorf("balance repo: get: %w", err)
	}
	return bal, nil
}

func (r *balanceDB) UpsertBalance(ctx context.Context, balance *domain.AccountBalance) error {
	_, err := r.db.NewInsert().Model(balance).
		On("CONFLICT (account_id) DO UPDATE").
		Set("balance = EXCLUDED.balance").
		Set("status = EXCLUDED.status").
		Set("as_of = EXCLUDED.as_of").
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("balance repo: upsert: %w", err)
	}
	return nil
}

func (r *balanceDB) ListByOwner(ctx context.Context, ownerID string) ([]*domain.AccountSummary, error) {
	var summaries []*domain.AccountSummary
	err := r.db.NewSelect().Model(&summaries).Where("owner_id = ?", ownerID).Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("balance repo: list by owner: %w", err)
	}
	return summaries, nil
}
