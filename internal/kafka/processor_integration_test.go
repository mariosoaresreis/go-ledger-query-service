package kafka

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/segmentio/kafka-go"
	"go-ledger-query-service/internal/domain"
	"go-ledger-query-service/internal/repository"
)

type inMemoryBalanceRepo struct {
	balances map[string]*domain.AccountBalance
}

func (r *inMemoryBalanceRepo) GetBalance(_ context.Context, accountID string) (*domain.AccountBalance, error) {
	b, ok := r.balances[accountID]
	if !ok {
		return nil, errNotFound(accountID)
	}
	return b, nil
}

func (r *inMemoryBalanceRepo) UpsertBalance(_ context.Context, balance *domain.AccountBalance) error {
	cpy := *balance
	r.balances[balance.AccountID] = &cpy
	return nil
}

func (r *inMemoryBalanceRepo) ListByOwner(_ context.Context, ownerID string) ([]*domain.AccountSummary, error) {
	out := make([]*domain.AccountSummary, 0)
	for _, b := range r.balances {
		if b.OwnerID != ownerID {
			continue
		}
		out = append(out, &domain.AccountSummary{
			AccountID: b.AccountID,
			OwnerID:   b.OwnerID,
			Currency:  b.Currency,
			Balance:   b.Balance,
			Status:    b.Status,
		})
	}
	return out, nil
}

type inMemoryTxnRepo struct {
	txns []*domain.Transaction
}

func (r *inMemoryTxnRepo) ListTransactions(_ context.Context, filter repository.TransactionFilter) ([]*domain.Transaction, int, error) {
	out := make([]*domain.Transaction, 0)
	for _, t := range r.txns {
		if t.AccountID == filter.AccountID {
			out = append(out, t)
		}
	}
	return out, len(out), nil
}

func (r *inMemoryTxnRepo) InsertTransaction(_ context.Context, tx *domain.Transaction) error {
	cpy := *tx
	r.txns = append(r.txns, &cpy)
	return nil
}

func (r *inMemoryTxnRepo) GetMonthlyTransactions(_ context.Context, accountID, month string) ([]*domain.Transaction, error) {
	out := make([]*domain.Transaction, 0)
	for _, t := range r.txns {
		if t.AccountID == accountID && t.CreatedAt.Format("2006-01") == month {
			out = append(out, t)
		}
	}
	return out, nil
}

type notFoundError string

func (e notFoundError) Error() string { return string(e) }

func errNotFound(id string) error {
	return notFoundError("account not found: " + id)
}

func TestProcessor_ProjectsCreateAndCreditEvents(t *testing.T) {
	balanceRepo := &inMemoryBalanceRepo{balances: map[string]*domain.AccountBalance{}}
	txnRepo := &inMemoryTxnRepo{txns: make([]*domain.Transaction, 0)}
	processor := NewProcessor(balanceRepo, txnRepo)

	accountID := "4f6f43c0-bd80-4d24-a4d6-c95f9506627d"
	seedBalance(t, processor, balanceRepo, accountID, "USD", 1250)

	bal, err := balanceRepo.GetBalance(context.Background(), accountID)
	if err != nil {
		t.Fatalf("balance lookup failed: %v", err)
	}
	if bal.Balance != 1250 {
		t.Fatalf("unexpected balance: got %d want 1250", bal.Balance)
	}
	if bal.Currency != "USD" {
		t.Fatalf("unexpected currency: got %s want USD", bal.Currency)
	}

	creditTxns := filterDir(txnRepo.txns, "CREDIT")
	if len(creditTxns) == 0 {
		t.Fatal("expected at least one CREDIT transaction")
	}
}

func TestProcessor_ProjectsDebitEvent(t *testing.T) {
	balanceRepo := &inMemoryBalanceRepo{balances: map[string]*domain.AccountBalance{}}
	txnRepo := &inMemoryTxnRepo{txns: make([]*domain.Transaction, 0)}
	processor := NewProcessor(balanceRepo, txnRepo)

	accountID := "aabbccdd-0001-0001-0001-aabbccddee01"
	seedBalance(t, processor, balanceRepo, accountID, "GBP", 5000)

	debitPayload, _ := json.Marshal(domain.AccountDebitedPayload{
		AccountID: accountID,
		Amount:    1200,
		Currency:  "GBP",
		Reference: "rent",
	})
	debited := domain.KafkaEvent{
		ID:          "evt-3",
		AggregateID: accountID,
		Version:     3,
		EventType:   domain.EventAccountDebited,
		Payload:     debitPayload,
		CreatedAt:   time.Now().UTC(),
	}
	if err := processor.Handle(context.Background(), toKafkaMessage(t, debited)); err != nil {
		t.Fatalf("debit event failed: %v", err)
	}

	bal, err := balanceRepo.GetBalance(context.Background(), accountID)
	if err != nil {
		t.Fatalf("balance lookup failed: %v", err)
	}
	if bal.Balance != 3800 {
		t.Fatalf("unexpected balance after debit: got %d want 3800", bal.Balance)
	}

	debitTxns := filterDir(txnRepo.txns, "DEBIT")
	if len(debitTxns) != 1 {
		t.Fatalf("unexpected DEBIT transaction count: got %d want 1", len(debitTxns))
	}
	if debitTxns[0].Amount != 1200 {
		t.Fatalf("unexpected debit amount: got %d want 1200", debitTxns[0].Amount)
	}
}

func TestProcessor_ProjectsStatusChange(t *testing.T) {
	balanceRepo := &inMemoryBalanceRepo{balances: map[string]*domain.AccountBalance{}}
	txnRepo := &inMemoryTxnRepo{txns: make([]*domain.Transaction, 0)}
	processor := NewProcessor(balanceRepo, txnRepo)

	accountID := "aabbccdd-0002-0002-0002-aabbccddee02"
	seedBalance(t, processor, balanceRepo, accountID, "EUR", 0)

	statusPayload, _ := json.Marshal(domain.AccountStatusChangedPayload{
		AccountID: accountID,
		OldStatus: domain.StatusActive,
		NewStatus: domain.StatusFrozen,
	})
	statusEvt := domain.KafkaEvent{
		ID:          "evt-4",
		AggregateID: accountID,
		Version:     2,
		EventType:   domain.EventAccountStatusChanged,
		Payload:     statusPayload,
		CreatedAt:   time.Now().UTC(),
	}
	if err := processor.Handle(context.Background(), toKafkaMessage(t, statusEvt)); err != nil {
		t.Fatalf("status change event failed: %v", err)
	}

	bal, err := balanceRepo.GetBalance(context.Background(), accountID)
	if err != nil {
		t.Fatalf("balance lookup failed: %v", err)
	}
	if bal.Status != domain.StatusFrozen {
		t.Fatalf("unexpected status: got %s want FROZEN", bal.Status)
	}
}

func TestProcessor_TransferSaga_BothBalancesUpdated(t *testing.T) {
	srcID := "src-0001-0001-0001-000000000001"
	dstID := "dst-0002-0002-0002-000000000002"

	// Pre-populate balance repo directly so no seed transactions pollute txnRepo.
	balanceRepo := &inMemoryBalanceRepo{balances: map[string]*domain.AccountBalance{
		srcID: {AccountID: srcID, OwnerID: "owner-src", Currency: "USD", Balance: 10000, Status: domain.StatusActive},
		dstID: {AccountID: dstID, OwnerID: "owner-dst", Currency: "USD", Balance: 500, Status: domain.StatusActive},
	}}
	txnRepo := &inMemoryTxnRepo{txns: make([]*domain.Transaction, 0)}
	processor := NewProcessor(balanceRepo, txnRepo)

	// Source is debited.
	debitPayload, _ := json.Marshal(domain.AccountDebitedPayload{
		AccountID: srcID, Amount: 3000, Currency: "USD", Reference: "TRANSFER:some-transfer-id",
	})
	if err := processor.Handle(context.Background(), toKafkaMessage(t, domain.KafkaEvent{
		ID: "evt-debit", AggregateID: srcID, Version: 3,
		EventType: domain.EventAccountDebited, Payload: debitPayload, CreatedAt: time.Now().UTC(),
	})); err != nil {
		t.Fatalf("debit event for transfer failed: %v", err)
	}

	// Target is credited.
	creditPayload, _ := json.Marshal(domain.AccountCreditedPayload{
		AccountID: dstID, Amount: 3000, Currency: "USD", Reference: "TRANSFER:some-transfer-id",
	})
	if err := processor.Handle(context.Background(), toKafkaMessage(t, domain.KafkaEvent{
		ID: "evt-credit", AggregateID: dstID, Version: 2,
		EventType: domain.EventAccountCredited, Payload: creditPayload, CreatedAt: time.Now().UTC(),
	})); err != nil {
		t.Fatalf("credit event for transfer failed: %v", err)
	}

	srcBal, err := balanceRepo.GetBalance(context.Background(), srcID)
	if err != nil {
		t.Fatalf("src balance lookup failed: %v", err)
	}
	if srcBal.Balance != 7000 {
		t.Fatalf("unexpected src balance: got %d want 7000", srcBal.Balance)
	}

	dstBal, err := balanceRepo.GetBalance(context.Background(), dstID)
	if err != nil {
		t.Fatalf("dst balance lookup failed: %v", err)
	}
	if dstBal.Balance != 3500 {
		t.Fatalf("unexpected dst balance: got %d want 3500", dstBal.Balance)
	}

	// Exactly one DEBIT (src) and one CREDIT (dst) — no seed noise.
	srcTxns := filterAccount(txnRepo.txns, srcID)
	dstTxns := filterAccount(txnRepo.txns, dstID)
	if debitTxns := filterDir(srcTxns, "DEBIT"); len(debitTxns) != 1 {
		t.Fatalf("expected 1 DEBIT on src, got %d", len(debitTxns))
	}
	if creditTxns := filterDir(dstTxns, "CREDIT"); len(creditTxns) != 1 {
		t.Fatalf("expected 1 CREDIT on dst, got %d", len(creditTxns))
	}
}

// ── helpers ────────────────────────────────────────────────────────────────────

// seedBalance emits ACCOUNT_CREATED + ACCOUNT_CREDITED to bootstrap the read model.
func seedBalance(t *testing.T, p *Processor, repo *inMemoryBalanceRepo, accountID, currency string, amount int64) {
	t.Helper()
	ownerID := "owner-" + accountID

	createdPayload, _ := json.Marshal(domain.AccountCreatedPayload{
		AccountID: accountID,
		OwnerID:   ownerID,
		Currency:  currency,
	})
	if err := p.Handle(context.Background(), toKafkaMessage(t, domain.KafkaEvent{
		ID:          "seed-create-" + accountID,
		AggregateID: accountID,
		Version:     1,
		EventType:   domain.EventAccountCreated,
		Payload:     createdPayload,
		CreatedAt:   time.Now().UTC(),
	})); err != nil {
		t.Fatalf("seed create failed: %v", err)
	}

	if amount > 0 {
		creditPayload, _ := json.Marshal(domain.AccountCreditedPayload{
			AccountID: accountID,
			Amount:    amount,
			Currency:  currency,
			Reference: "seed",
		})
		if err := p.Handle(context.Background(), toKafkaMessage(t, domain.KafkaEvent{
			ID:          "seed-credit-" + accountID,
			AggregateID: accountID,
			Version:     2,
			EventType:   domain.EventAccountCredited,
			Payload:     creditPayload,
			CreatedAt:   time.Now().UTC(),
		})); err != nil {
			t.Fatalf("seed credit failed: %v", err)
		}
	}
	_ = repo
}

func filterDir(txns []*domain.Transaction, dir string) []*domain.Transaction {
	out := make([]*domain.Transaction, 0)
	for _, t := range txns {
		if t.Direction == dir {
			out = append(out, t)
		}
	}
	return out
}

func filterAccount(txns []*domain.Transaction, accountID string) []*domain.Transaction {
	out := make([]*domain.Transaction, 0)
	for _, t := range txns {
		if t.AccountID == accountID {
			out = append(out, t)
		}
	}
	return out
}

func toKafkaMessage(t *testing.T, evt domain.KafkaEvent) kafka.Message {
	t.Helper()
	b, err := json.Marshal(evt)
	if err != nil {
		t.Fatalf("marshal event: %v", err)
	}
	return kafka.Message{Value: b}
}
