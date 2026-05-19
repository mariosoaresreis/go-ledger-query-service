package api_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"go-ledger-query-service/internal/api"
	"go-ledger-query-service/internal/domain"
	"go-ledger-query-service/internal/repository"

	"github.com/gin-gonic/gin"
)

// ── mock service ─────────────────────────────────────────────────────────────

type mockQueryService struct {
	balance  *domain.AccountBalance
	balErr   error
	txns     []*domain.Transaction
	txnTotal int
	txnErr   error
	stmt     *domain.Statement
	stmtErr  error
	accounts []*domain.AccountSummary
	acctErr  error
}

func (m *mockQueryService) GetBalance(_ context.Context, _ string) (*domain.AccountBalance, error) {
	return m.balance, m.balErr
}
func (m *mockQueryService) ListTransactions(_ context.Context, _ repository.TransactionFilter) ([]*domain.Transaction, int, error) {
	return m.txns, m.txnTotal, m.txnErr
}
func (m *mockQueryService) GetStatement(_ context.Context, _, _ string) (*domain.Statement, error) {
	return m.stmt, m.stmtErr
}
func (m *mockQueryService) ListAccountsByOwner(_ context.Context, _ string) ([]*domain.AccountSummary, error) {
	return m.accounts, m.acctErr
}

// ── helpers ──────────────────────────────────────────────────────────────────

func newTestRouter(svc *mockQueryService) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := api.NewAccountQueryHandler(svc)
	v1 := r.Group("/api/v1")
	v1.GET("/accounts", h.ListAccountsByOwner)
	v1.GET("/accounts/:accountId/balance", h.GetBalance)
	v1.GET("/accounts/:accountId/transactions", h.ListTransactions)
	v1.GET("/accounts/:accountId/statement", h.GetStatement)
	return r
}

func doGet(t *testing.T, router *gin.Engine, path string) *httptest.ResponseRecorder {
	t.Helper()
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, path, nil)
	router.ServeHTTP(w, req)
	return w
}

// ── GetBalance ────────────────────────────────────────────────────────────────

func TestGetBalance_200(t *testing.T) {
	svc := &mockQueryService{
		balance: &domain.AccountBalance{AccountID: "acc1", Balance: 750, Currency: "USD", Status: domain.StatusActive},
	}
	w := doGet(t, newTestRouter(svc), "/api/v1/accounts/acc1/balance")

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var got domain.AccountBalance
	if err := json.NewDecoder(w.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.Balance != 750 {
		t.Errorf("balance: want 750, got %d", got.Balance)
	}
}

func TestGetBalance_404(t *testing.T) {
	svc := &mockQueryService{balErr: errors.New("account acc99 not found")}
	w := doGet(t, newTestRouter(svc), "/api/v1/accounts/acc99/balance")
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

// ── ListTransactions ──────────────────────────────────────────────────────────

func TestListTransactions_200(t *testing.T) {
	now := time.Now()
	svc := &mockQueryService{
		txns: []*domain.Transaction{
			{ID: "t1", AccountID: "acc1", Direction: "CREDIT", Amount: 100, CreatedAt: now},
		},
		txnTotal: 1,
	}
	w := doGet(t, newTestRouter(svc), "/api/v1/accounts/acc1/transactions?page=0&size=10")
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var resp api.PaginatedResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.TotalCount != 1 {
		t.Errorf("totalCount: want 1, got %d", resp.TotalCount)
	}
}

func TestListTransactions_500(t *testing.T) {
	svc := &mockQueryService{txnErr: errors.New("db down")}
	w := doGet(t, newTestRouter(svc), "/api/v1/accounts/acc1/transactions")
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

// ── GetStatement ──────────────────────────────────────────────────────────────

func TestGetStatement_200(t *testing.T) {
	svc := &mockQueryService{
		stmt: &domain.Statement{
			AccountID:      "acc1",
			Month:          "2026-05",
			OpeningBalance: 1000,
			ClosingBalance: 1200,
			Entries:        []domain.Transaction{},
		},
	}
	w := doGet(t, newTestRouter(svc), "/api/v1/accounts/acc1/statement?month=2026-05")
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var got domain.Statement
	if err := json.NewDecoder(w.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.OpeningBalance != 1000 || got.ClosingBalance != 1200 {
		t.Errorf("balances wrong: opening=%d closing=%d", got.OpeningBalance, got.ClosingBalance)
	}
}

func TestGetStatement_400_missingMonth(t *testing.T) {
	svc := &mockQueryService{}
	w := doGet(t, newTestRouter(svc), "/api/v1/accounts/acc1/statement")
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestGetStatement_500(t *testing.T) {
	svc := &mockQueryService{stmtErr: errors.New("db down")}
	w := doGet(t, newTestRouter(svc), "/api/v1/accounts/acc1/statement?month=2026-05")
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

// ── ListAccountsByOwner ───────────────────────────────────────────────────────

func TestListAccountsByOwner_200(t *testing.T) {
	svc := &mockQueryService{
		accounts: []*domain.AccountSummary{
			{AccountID: "acc1", OwnerID: "owner1", Currency: "USD", Balance: 100},
		},
	}
	w := doGet(t, newTestRouter(svc), "/api/v1/accounts?ownerId=owner1")
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestListAccountsByOwner_400_missingOwner(t *testing.T) {
	svc := &mockQueryService{}
	w := doGet(t, newTestRouter(svc), "/api/v1/accounts")
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestListAccountsByOwner_500(t *testing.T) {
	svc := &mockQueryService{acctErr: errors.New("db down")}
	w := doGet(t, newTestRouter(svc), "/api/v1/accounts?ownerId=owner1")
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}
