package kafka

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/segmentio/kafka-go"
	"github.com/sirupsen/logrus"
	"go-ledger-query-service/internal/domain"
	"go-ledger-query-service/internal/repository"
)

// Processor handles incoming Kafka events and updates the read model.
type Processor struct {
	balanceRepo     repository.BalanceRepository
	transactionRepo repository.TransactionRepository
}

// NewProcessor creates a new event processor.
func NewProcessor(balanceRepo repository.BalanceRepository, txRepo repository.TransactionRepository) *Processor {
	return &Processor{balanceRepo: balanceRepo, transactionRepo: txRepo}
}

// Handle dispatches a Kafka message to the correct projection handler.
func (p *Processor) Handle(ctx context.Context, msg kafka.Message) error {
	var event domain.KafkaEvent
	if err := json.Unmarshal(msg.Value, &event); err != nil {
		return fmt.Errorf("processor: unmarshal event: %w", err)
	}

	logrus.WithFields(logrus.Fields{
		"event_type":   event.EventType,
		"aggregate_id": event.AggregateID,
		"version":      event.Version,
	}).Debug("processor: received event")

	switch event.EventType {
	case domain.EventAccountCreated:
		return p.handleAccountCreated(ctx, event)
	case domain.EventAccountCredited:
		return p.handleAccountCredited(ctx, event)
	case domain.EventAccountDebited:
		return p.handleAccountDebited(ctx, event)
	case domain.EventAccountStatusChanged:
		return p.handleStatusChanged(ctx, event)
	default:
		logrus.WithField("event_type", event.EventType).Debug("processor: unhandled event type")
	}
	return nil
}

func (p *Processor) handleAccountCreated(ctx context.Context, event domain.KafkaEvent) error {
	var payload domain.AccountCreatedPayload
	if err := json.Unmarshal(event.Payload, &payload); err != nil {
		return fmt.Errorf("processor: unmarshal account created: %w", err)
	}

	bal := &domain.AccountBalance{
		AccountID: payload.AccountID,
		OwnerID:   payload.OwnerID,
		Currency:  payload.Currency,
		Balance:   0,
		Status:    domain.StatusActive,
		AsOf:      event.CreatedAt,
	}
	return p.balanceRepo.UpsertBalance(ctx, bal)
}

func (p *Processor) handleAccountCredited(ctx context.Context, event domain.KafkaEvent) error {
	var payload domain.AccountCreditedPayload
	if err := json.Unmarshal(event.Payload, &payload); err != nil {
		return fmt.Errorf("processor: unmarshal account credited: %w", err)
	}

	bal, err := p.balanceRepo.GetBalance(ctx, payload.AccountID)
	if err != nil {
		return fmt.Errorf("processor: get balance for credit: %w", err)
	}
	bal.Balance += payload.Amount
	bal.AsOf = event.CreatedAt
	if err := p.balanceRepo.UpsertBalance(ctx, bal); err != nil {
		return err
	}

	txn := &domain.Transaction{
		ID:        event.ID,
		AccountID: payload.AccountID,
		EventType: event.EventType,
		Amount:    payload.Amount,
		Currency:  payload.Currency,
		Direction: "CREDIT",
		Reference: payload.Reference,
		CreatedAt: event.CreatedAt,
	}
	return p.transactionRepo.InsertTransaction(ctx, txn)
}

func (p *Processor) handleAccountDebited(ctx context.Context, event domain.KafkaEvent) error {
	var payload domain.AccountDebitedPayload
	if err := json.Unmarshal(event.Payload, &payload); err != nil {
		return fmt.Errorf("processor: unmarshal account debited: %w", err)
	}

	bal, err := p.balanceRepo.GetBalance(ctx, payload.AccountID)
	if err != nil {
		return fmt.Errorf("processor: get balance for debit: %w", err)
	}
	bal.Balance -= payload.Amount
	bal.AsOf = event.CreatedAt
	if err := p.balanceRepo.UpsertBalance(ctx, bal); err != nil {
		return err
	}

	txn := &domain.Transaction{
		ID:        event.ID,
		AccountID: payload.AccountID,
		EventType: event.EventType,
		Amount:    payload.Amount,
		Currency:  payload.Currency,
		Direction: "DEBIT",
		Reference: payload.Reference,
		CreatedAt: event.CreatedAt,
	}
	return p.transactionRepo.InsertTransaction(ctx, txn)
}

func (p *Processor) handleStatusChanged(ctx context.Context, event domain.KafkaEvent) error {
	var payload domain.AccountStatusChangedPayload
	if err := json.Unmarshal(event.Payload, &payload); err != nil {
		return fmt.Errorf("processor: unmarshal status changed: %w", err)
	}

	bal, err := p.balanceRepo.GetBalance(ctx, payload.AccountID)
	if err != nil {
		return fmt.Errorf("processor: get balance for status change: %w", err)
	}
	bal.Status = payload.NewStatus
	bal.AsOf = time.Now().UTC()
	return p.balanceRepo.UpsertBalance(ctx, bal)
}
