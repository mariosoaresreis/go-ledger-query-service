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
	transactor      repository.Transactor
}

// NewProcessor creates a new event processor.
func NewProcessor(
	balanceRepo repository.BalanceRepository,
	txRepo repository.TransactionRepository,
	transactor repository.Transactor,
) *Processor {
	return &Processor{
		balanceRepo:     balanceRepo,
		transactionRepo: txRepo,
		transactor:      transactor,
	}
}

// Handle dispatches a Kafka message to the correct projection handler.
func (p *Processor) Handle(ctx context.Context, msg kafka.Message) error {
	var event domain.KafkaEvent
	if err := json.Unmarshal(msg.Value, &event); err != nil {
		return fmt.Errorf("processor: unmarshal event: %w", err)
	}

	// Command service currently publishes PascalCase JSON keys (e.g., EventType),
	// while this service historically read snake_case. Support both formats.
	if event.EventType == "" {
		var legacy struct {
			ID          string           `json:"ID"`
			AggregateID string           `json:"AggregateID"`
			Version     int64            `json:"Version"`
			EventType   domain.EventType `json:"EventType"`
			Payload     []byte           `json:"Payload"`
			CreatedAt   time.Time        `json:"CreatedAt"`
		}
		if err := json.Unmarshal(msg.Value, &legacy); err != nil {
			return fmt.Errorf("processor: unmarshal legacy event envelope: %w", err)
		}
		event = domain.KafkaEvent{
			ID:          legacy.ID,
			AggregateID: legacy.AggregateID,
			Version:     legacy.Version,
			EventType:   legacy.EventType,
			Payload:     legacy.Payload,
			CreatedAt:   legacy.CreatedAt,
		}
	}

	if event.EventType == "" {
		return fmt.Errorf("processor: event type missing")
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

// handleAccountCredited atomically updates the balance projection AND inserts the
// transaction record in one database transaction so a crash between the two writes
// cannot leave the read model in an inconsistent state.
func (p *Processor) handleAccountCredited(ctx context.Context, event domain.KafkaEvent) error {
	var payload domain.AccountCreditedPayload
	if err := json.Unmarshal(event.Payload, &payload); err != nil {
		return fmt.Errorf("processor: unmarshal account credited: %w", err)
	}

	return p.transactor.RunInTx(ctx, func(ctx context.Context, balRepo repository.BalanceRepository, txRepo repository.TransactionRepository) error {
		bal, err := balRepo.GetBalance(ctx, payload.AccountID)
		if err != nil {
			return fmt.Errorf("processor: get balance for credit: %w", err)
		}
		bal.Balance += payload.Amount
		bal.AsOf = event.CreatedAt
		if err := balRepo.UpsertBalance(ctx, bal); err != nil {
			return err
		}
		return txRepo.InsertTransaction(ctx, &domain.Transaction{
			ID:        event.ID,
			AccountID: payload.AccountID,
			EventType: event.EventType,
			Amount:    payload.Amount,
			Currency:  payload.Currency,
			Direction: "CREDIT",
			Reference: payload.Reference,
			CreatedAt: event.CreatedAt,
		})
	})
}

// handleAccountDebited is the mirror of handleAccountCredited — also atomic.
func (p *Processor) handleAccountDebited(ctx context.Context, event domain.KafkaEvent) error {
	var payload domain.AccountDebitedPayload
	if err := json.Unmarshal(event.Payload, &payload); err != nil {
		return fmt.Errorf("processor: unmarshal account debited: %w", err)
	}

	return p.transactor.RunInTx(ctx, func(ctx context.Context, balRepo repository.BalanceRepository, txRepo repository.TransactionRepository) error {
		bal, err := balRepo.GetBalance(ctx, payload.AccountID)
		if err != nil {
			return fmt.Errorf("processor: get balance for debit: %w", err)
		}
		bal.Balance -= payload.Amount
		bal.AsOf = event.CreatedAt
		if err := balRepo.UpsertBalance(ctx, bal); err != nil {
			return err
		}
		return txRepo.InsertTransaction(ctx, &domain.Transaction{
			ID:        event.ID,
			AccountID: payload.AccountID,
			EventType: event.EventType,
			Amount:    payload.Amount,
			Currency:  payload.Currency,
			Direction: "DEBIT",
			Reference: payload.Reference,
			CreatedAt: event.CreatedAt,
		})
	})
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
