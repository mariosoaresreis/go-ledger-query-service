package repository

import (
	"context"
	"regexp"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"go-ledger-query-service/internal/domain"
)

func TestTransactionRepository_ListTransactions_WithFiltersAndPagination(t *testing.T) {
	db, mock, cleanup := newMockDB(t)
	defer cleanup()

	repo := NewTransactionRepository(db)
	filter := TransactionFilter{
		AccountID: "acc-1",
		From:      "2026-04-01",
		To:        "2026-04-30",
		Direction: "CREDIT",
		Page:      1,
		Size:      10,
	}

	countQuery := regexp.QuoteMeta("SELECT COUNT(1) FROM transactions WHERE account_id = $1 AND created_at >= $2 AND created_at <= $3 AND direction = $4")
	mock.ExpectQuery(countQuery).
		WithArgs("acc-1", "2026-04-01", "2026-04-30 23:59:59", "CREDIT").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(2))

	listQuery := regexp.QuoteMeta("SELECT id, account_id, event_type, amount, currency, direction, reference, created_at FROM transactions WHERE account_id = $1 AND created_at >= $2 AND created_at <= $3 AND direction = $4 ORDER BY created_at DESC LIMIT $5 OFFSET $6")
	now := time.Now().UTC()
	rows := sqlmock.NewRows([]string{"id", "account_id", "event_type", "amount", "currency", "direction", "reference", "created_at"}).
		AddRow("evt-1", "acc-1", "ACCOUNT_CREDITED", int64(10), "USD", "CREDIT", "ref-1", now).
		AddRow("evt-2", "acc-1", "ACCOUNT_CREDITED", int64(20), "USD", "CREDIT", "ref-2", now.Add(-time.Minute))

	mock.ExpectQuery(listQuery).
		WithArgs("acc-1", "2026-04-01", "2026-04-30 23:59:59", "CREDIT", 10, 10).
		WillReturnRows(rows)

	txns, total, err := repo.ListTransactions(context.Background(), filter)
	if err != nil {
		t.Fatalf("ListTransactions returned error: %v", err)
	}
	if total != 2 {
		t.Fatalf("unexpected total: got %d want 2", total)
	}
	if len(txns) != 2 {
		t.Fatalf("unexpected transaction len: got %d want 2", len(txns))
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet SQL expectations: %v", err)
	}
}

func TestTransactionRepository_ListTransactions_DefaultPagination(t *testing.T) {
	db, mock, cleanup := newMockDB(t)
	defer cleanup()

	repo := NewTransactionRepository(db)
	filter := TransactionFilter{AccountID: "acc-2", Page: -1, Size: 0}

	countQuery := regexp.QuoteMeta("SELECT COUNT(1) FROM transactions WHERE account_id = $1")
	mock.ExpectQuery(countQuery).
		WithArgs("acc-2").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))

	listQuery := regexp.QuoteMeta("SELECT id, account_id, event_type, amount, currency, direction, reference, created_at FROM transactions WHERE account_id = $1 ORDER BY created_at DESC LIMIT $2 OFFSET $3")
	mock.ExpectQuery(listQuery).
		WithArgs("acc-2", 20, 0).
		WillReturnRows(sqlmock.NewRows([]string{"id", "account_id", "event_type", "amount", "currency", "direction", "reference", "created_at"}))

	txns, total, err := repo.ListTransactions(context.Background(), filter)
	if err != nil {
		t.Fatalf("ListTransactions returned error: %v", err)
	}
	if total != 0 {
		t.Fatalf("unexpected total: got %d want 0", total)
	}
	if len(txns) != 0 {
		t.Fatalf("unexpected transaction len: got %d want 0", len(txns))
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet SQL expectations: %v", err)
	}
}

func TestTransactionRepository_GetMonthlyTransactions(t *testing.T) {
	db, mock, cleanup := newMockDB(t)
	defer cleanup()

	repo := NewTransactionRepository(db)

	query := regexp.QuoteMeta(`
		SELECT id, account_id, event_type, amount, currency, direction, reference, created_at
		FROM transactions
		WHERE account_id = $1
		  AND TO_CHAR(created_at, 'YYYY-MM') = $2
		ORDER BY created_at ASC
	`)
	now := time.Now().UTC()
	rows := sqlmock.NewRows([]string{"id", "account_id", "event_type", "amount", "currency", "direction", "reference", "created_at"}).
		AddRow("evt-month", "acc-3", "ACCOUNT_DEBITED", int64(40), "USD", "DEBIT", "rent", now)

	mock.ExpectQuery(query).
		WithArgs("acc-3", "2026-04").
		WillReturnRows(rows)

	txns, err := repo.GetMonthlyTransactions(context.Background(), "acc-3", "2026-04")
	if err != nil {
		t.Fatalf("GetMonthlyTransactions returned error: %v", err)
	}
	if len(txns) != 1 {
		t.Fatalf("unexpected transaction len: got %d want 1", len(txns))
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet SQL expectations: %v", err)
	}
}

func TestTransactionRepository_InsertTransaction(t *testing.T) {
	db, mock, cleanup := newMockDB(t)
	defer cleanup()

	repo := NewTransactionRepository(db)
	now := time.Now().UTC()
	txn := &domain.Transaction{
		ID:        "evt-10",
		AccountID: "acc-10",
		EventType: domain.EventAccountCredited,
		Amount:    300,
		Currency:  "USD",
		Direction: "CREDIT",
		Reference: "salary",
		CreatedAt: now,
	}

	execQuery := regexp.QuoteMeta(`
		INSERT INTO transactions (id, account_id, event_type, amount, currency, direction, reference, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		ON CONFLICT (id) DO NOTHING
	`)
	mock.ExpectExec(execQuery).
		WithArgs("evt-10", "acc-10", domain.EventAccountCredited, int64(300), "USD", "CREDIT", "salary", now).
		WillReturnResult(sqlmock.NewResult(1, 1))

	if err := repo.InsertTransaction(context.Background(), txn); err != nil {
		t.Fatalf("InsertTransaction returned error: %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet SQL expectations: %v", err)
	}
}
