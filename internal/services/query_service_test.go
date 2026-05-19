package services_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"go-ledger-query-service/internal/domain"
	"go-ledger-query-service/internal/repository"
	"go-ledger-query-service/internal/services"
)

// ── thin in-process mocks ────────────────────────────────────────────────────

type mockBalanceRepo struct {
	balance *domain.AccountBalance
	err     error
	list    []*domain.AccountSummary
	listErr error
}

func (m *mockBalanceRepo) GetBalance(_ context.Context, _ string) (*domain.AccountBalance, error) {
	return m.balance, m.err
}
func (m *mockBalanceRepo) UpsertBalance(_ context.Context, _ *domain.AccountBalance) error {
	return m.err
}
func (m *mockBalanceRepo) ListByOwner(_ context.Context, _ string) ([]*domain.AccountSummary, error) {
	return m.list, m.listErr
}

type mockTransactionRepo struct {
	txns        []*domain.Transaction
	txnsErr     error
	total       int
	sumBefore   int64
	sumBeforeErr error
}

func (m *mockTransactionRepo) ListTransactions(_ context.Context, _ repository.TransactionFilter) ([]*domain.Transaction, int, error) {
	return m.txns, m.total, m.txnsErr
}
func (m *mockTransactionRepo) InsertTransaction(_ context.Context, _ *domain.Transaction) error {
	return nil
}
func (m *mockTransactionRepo) GetMonthlyTransactions(_ context.Context, _, _ string) ([]*domain.Transaction, error) {
	return m.txns, m.txnsErr
}
func (m *mockTransactionRepo) SumTransactionsBefore(_ context.Context, _ string, _ string) (int64, error) {
	return m.sumBefore, m.sumBeforeErr
}

// ── tests ────────────────────────────────────────────────────────────────────

func TestGetBalance_success(t *testing.T) {
	bal := &domain.AccountBalance{AccountID: "acc1", Balance: 500, Currency: "USD", Status: domain.StatusActive}
	svc := services.NewQueryService(&mockBalanceRepo{balance: bal}, &mockTransactionRepo{})

	got, err := svc.GetBalance(context.Background(), "acc1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Balance != 500 {
		t.Errorf("expected balance 500, got %d", got.Balance)
	}
}

func TestGetBalance_notFound(t *testing.T) {
	svc := services.NewQueryService(
		&mockBalanceRepo{err: errors.New("account acc99 not found")},
		&mockTransactionRepo{},
	)
	_, err := svc.GetBalance(context.Background(), "acc99")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestListTransactions_returnsPagedResults(t *testing.T) {
	now := time.Now()
	txns := []*domain.Transaction{
		{ID: "t1", AccountID: "acc1", Direction: "CREDIT", Amount: 100, CreatedAt: now},
		{ID: "t2", AccountID: "acc1", Direction: "DEBIT", Amount: 50, CreatedAt: now},
	}
	svc := services.NewQueryService(&mockBalanceRepo{}, &mockTransactionRepo{txns: txns, total: 2})

	got, total, err := svc.ListTransactions(context.Background(), repository.TransactionFilter{AccountID: "acc1", Page: 0, Size: 20})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if total != 2 {
		t.Errorf("expected total 2, got %d", total)
	}
	if len(got) != 2 {
		t.Errorf("expected 2 results, got %d", len(got))
	}
}

func TestListTransactions_repoError(t *testing.T) {
	svc := services.NewQueryService(&mockBalanceRepo{}, &mockTransactionRepo{txnsErr: errors.New("db down")})
	_, _, err := svc.ListTransactions(context.Background(), repository.TransactionFilter{AccountID: "acc1"})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestGetStatement_correctBalances(t *testing.T) {
	// Opening = 1000 (sum of pre-month txns)
	// This month: +200 credit, -80 debit  →  closing = 1000 + 200 - 80 = 1120
	now := time.Now()
	txns := []*domain.Transaction{
		{ID: "t1", AccountID: "acc1", Direction: "CREDIT", Amount: 200, CreatedAt: now},
		{ID: "t2", AccountID: "acc1", Direction: "DEBIT", Amount: 80, CreatedAt: now},
	}
	svc := services.NewQueryService(
		&mockBalanceRepo{},
		&mockTransactionRepo{txns: txns, sumBefore: 1000},
	)

	stmt, err := svc.GetStatement(context.Background(), "acc1", "2026-05")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stmt.OpeningBalance != 1000 {
		t.Errorf("opening: want 1000, got %d", stmt.OpeningBalance)
	}
	if stmt.ClosingBalance != 1120 {
		t.Errorf("closing: want 1120, got %d", stmt.ClosingBalance)
	}
	if len(stmt.Entries) != 2 {
		t.Errorf("entries: want 2, got %d", len(stmt.Entries))
	}
}

func TestGetStatement_sumBeforeError(t *testing.T) {
	svc := services.NewQueryService(
		&mockBalanceRepo{},
		&mockTransactionRepo{sumBeforeErr: errors.New("db error")},
	)
	_, err := svc.GetStatement(context.Background(), "acc1", "2026-05")
	if err == nil {
		t.Fatal("expected error from SumTransactionsBefore, got nil")
	}
}

func TestGetStatement_monthlyTxnsError(t *testing.T) {
	svc := services.NewQueryService(
		&mockBalanceRepo{},
		&mockTransactionRepo{txnsErr: errors.New("db error")},
	)
	_, err := svc.GetStatement(context.Background(), "acc1", "2026-05")
	if err == nil {
		t.Fatal("expected error from GetMonthlyTransactions, got nil")
	}
}

func TestListAccountsByOwner_success(t *testing.T) {
	summaries := []*domain.AccountSummary{
		{AccountID: "acc1", OwnerID: "owner1", Currency: "USD", Balance: 100},
		{AccountID: "acc2", OwnerID: "owner1", Currency: "EUR", Balance: 200},
	}
	svc := services.NewQueryService(&mockBalanceRepo{list: summaries}, &mockTransactionRepo{})

	got, err := svc.ListAccountsByOwner(context.Background(), "owner1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 2 {
		t.Errorf("expected 2 accounts, got %d", len(got))
	}
}

func TestListAccountsByOwner_repoError(t *testing.T) {
	svc := services.NewQueryService(
		&mockBalanceRepo{listErr: errors.New("db down")},
		&mockTransactionRepo{},
	)
	_, err := svc.ListAccountsByOwner(context.Background(), "owner1")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

