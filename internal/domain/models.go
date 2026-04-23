package domain

import "time"

// EventType mirrors the command service types for deserialization.
type EventType string

const (
	EventAccountCreated       EventType = "ACCOUNT_CREATED"
	EventAccountCredited      EventType = "ACCOUNT_CREDITED"
	EventAccountDebited       EventType = "ACCOUNT_DEBITED"
	EventAccountStatusChanged EventType = "ACCOUNT_STATUS_CHANGED"
	EventTransferInitiated    EventType = "TRANSFER_INITIATED"
	EventTransferCompleted    EventType = "TRANSFER_COMPLETED"
	EventTransferReversed     EventType = "TRANSFER_REVERSED"
)

// AccountStatus mirrors the command service.
type AccountStatus string

const (
	StatusActive AccountStatus = "ACTIVE"
	StatusFrozen AccountStatus = "FROZEN"
	StatusClosed AccountStatus = "CLOSED"
)

// KafkaEvent is the envelope received from the ledger.events Kafka topic.
type KafkaEvent struct {
	ID          string    `json:"id"`
	AggregateID string    `json:"aggregate_id"`
	Version     int64     `json:"version"`
	EventType   EventType `json:"event_type"`
	Payload     []byte    `json:"payload"`
	CreatedAt   time.Time `json:"created_at"`
}

// AccountBalance is the read-model projection for GET /accounts/{id}/balance.
type AccountBalance struct {
	AccountID string        `bun:"account_id,pk"     json:"accountId"`
	OwnerID   string        `bun:"owner_id"          json:"ownerId"`
	Currency  string        `bun:"currency"          json:"currency"`
	Balance   int64         `bun:"balance"           json:"balance"`
	Status    AccountStatus `bun:"status"            json:"status"`
	AsOf      time.Time     `bun:"as_of"             json:"asOf"`
}

func (AccountBalance) TableName() string { return "account_balances" }

// Transaction is the read-model projection for the transaction history.
type Transaction struct {
	ID        string    `bun:"id,pk"         json:"id"`
	AccountID string    `bun:"account_id"    json:"accountId"`
	EventType EventType `bun:"event_type"    json:"eventType"`
	Amount    int64     `bun:"amount"        json:"amount"`
	Currency  string    `bun:"currency"      json:"currency"`
	Direction string    `bun:"direction"     json:"direction"` // CREDIT | DEBIT
	Reference string    `bun:"reference"     json:"reference"`
	CreatedAt time.Time `bun:"created_at"    json:"createdAt"`
}

func (Transaction) TableName() string { return "transactions" }

// Statement is computed on-the-fly from transactions.
type Statement struct {
	AccountID      string        `json:"accountId"`
	Month          string        `json:"month"` // YYYY-MM
	OpeningBalance int64         `json:"openingBalance"`
	ClosingBalance int64         `json:"closingBalance"`
	Entries        []Transaction `json:"entries"`
}

// AccountSummary is returned by the list-by-owner query.
type AccountSummary struct {
	AccountID string        `bun:"account_id" json:"accountId"`
	OwnerID   string        `bun:"owner_id"   json:"ownerId"`
	Currency  string        `bun:"currency"   json:"currency"`
	Balance   int64         `bun:"balance"    json:"balance"`
	Status    AccountStatus `bun:"status"     json:"status"`
}

func (AccountSummary) TableName() string { return "account_balances" }

// Payload types for event deserialization.
type AccountCreatedPayload struct {
	AccountID string `json:"accountId"`
	OwnerID   string `json:"ownerId"`
	Currency  string `json:"currency"`
}

type AccountCreditedPayload struct {
	AccountID string `json:"accountId"`
	Amount    int64  `json:"amount"`
	Currency  string `json:"currency"`
	Reference string `json:"reference"`
}

type AccountDebitedPayload struct {
	AccountID string `json:"accountId"`
	Amount    int64  `json:"amount"`
	Currency  string `json:"currency"`
	Reference string `json:"reference"`
}

type AccountStatusChangedPayload struct {
	AccountID string        `json:"accountId"`
	OldStatus AccountStatus `json:"oldStatus"`
	NewStatus AccountStatus `json:"newStatus"`
}
