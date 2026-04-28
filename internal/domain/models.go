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
	AccountID string        `db:"account_id" json:"accountId"`
	OwnerID   string        `db:"owner_id"   json:"ownerId"`
	Currency  string        `db:"currency"   json:"currency"`
	Balance   int64         `db:"balance"    json:"balance"`
	Status    AccountStatus `db:"status"     json:"status"`
	AsOf      time.Time     `db:"as_of"      json:"asOf"`
}

// Transaction is the read-model projection for the transaction history.
type Transaction struct {
	ID        string    `db:"id"         json:"id"`
	AccountID string    `db:"account_id" json:"accountId"`
	EventType EventType `db:"event_type" json:"eventType"`
	Amount    int64     `db:"amount"     json:"amount"`
	Currency  string    `db:"currency"   json:"currency"`
	Direction string    `db:"direction"  json:"direction"` // CREDIT | DEBIT
	Reference string    `db:"reference"  json:"reference"`
	CreatedAt time.Time `db:"created_at" json:"createdAt"`
}

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
	AccountID string        `db:"account_id" json:"accountId"`
	OwnerID   string        `db:"owner_id"   json:"ownerId"`
	Currency  string        `db:"currency"   json:"currency"`
	Balance   int64         `db:"balance"    json:"balance"`
	Status    AccountStatus `db:"status"     json:"status"`
}

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
