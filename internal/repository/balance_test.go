package repository

import (
	"context"
	"regexp"
	"strings"
	"testing"
	"time"

	"go-ledger-query-service/internal/domain"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
)

func TestBalanceRepository_GetBalance(t *testing.T) {
	db, mock, cleanup := newMockDB(t)
	defer cleanup()

	repo := NewBalanceRepository(db)
	now := time.Now().UTC()

	rows := sqlmock.NewRows([]string{"account_id", "owner_id", "currency", "balance", "status", "as_of"}).
		AddRow("acc-1", "owner-1", "USD", int64(1000), "ACTIVE", now)

	mock.ExpectQuery(regexp.QuoteMeta(`
		SELECT account_id, owner_id, currency, balance, status, as_of
		FROM account_balances
		WHERE account_id = $1
	`)).
		WithArgs("acc-1").
		WillReturnRows(rows)

	bal, err := repo.GetBalance(context.Background(), "acc-1")
	if err != nil {
		t.Fatalf("GetBalance returned error: %v", err)
	}
	if bal.AccountID != "acc-1" || bal.OwnerID != "owner-1" || bal.Balance != 1000 {
		t.Fatalf("unexpected balance: %+v", bal)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet SQL expectations: %v", err)
	}
}

func TestBalanceRepository_GetBalanceNotFound(t *testing.T) {
	db, mock, cleanup := newMockDB(t)
	defer cleanup()

	repo := NewBalanceRepository(db)

	mock.ExpectQuery(regexp.QuoteMeta(`
		SELECT account_id, owner_id, currency, balance, status, as_of
		FROM account_balances
		WHERE account_id = $1
	`)).
		WithArgs("missing").
		WillReturnError(noRowsError())

	_, err := repo.GetBalance(context.Background(), "missing")
	if err == nil {
		t.Fatal("expected not-found error")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Fatalf("unexpected error message: %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet SQL expectations: %v", err)
	}
}

func TestBalanceRepository_UpsertBalance(t *testing.T) {
	db, mock, cleanup := newMockDB(t)
	defer cleanup()

	repo := NewBalanceRepository(db)
	now := time.Now().UTC()
	balance := &domain.AccountBalance{
		AccountID: "acc-1",
		OwnerID:   "owner-1",
		Currency:  "USD",
		Balance:   1250,
		Status:    domain.StatusActive,
		AsOf:      now,
	}

	mock.ExpectExec(regexp.QuoteMeta(`
		INSERT INTO account_balances (account_id, owner_id, currency, balance, status, as_of)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (account_id) DO UPDATE
		SET owner_id = EXCLUDED.owner_id,
		    currency = EXCLUDED.currency,
		    balance = EXCLUDED.balance,
		    status = EXCLUDED.status,
		    as_of = EXCLUDED.as_of
	`)).
		WithArgs("acc-1", "owner-1", "USD", int64(1250), domain.StatusActive, now).
		WillReturnResult(sqlmock.NewResult(1, 1))

	if err := repo.UpsertBalance(context.Background(), balance); err != nil {
		t.Fatalf("UpsertBalance returned error: %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet SQL expectations: %v", err)
	}
}
