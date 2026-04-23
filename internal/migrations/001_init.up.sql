-- +migrate Up

CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- Denormalized account balance projection (rebuilt from events)
CREATE TABLE IF NOT EXISTS account_balances (
    account_id UUID        PRIMARY KEY,
    owner_id   UUID        NOT NULL,
    currency   CHAR(3)     NOT NULL,
    balance    BIGINT      NOT NULL DEFAULT 0,
    status     VARCHAR(10) NOT NULL DEFAULT 'ACTIVE',
    as_of      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_account_balances_owner ON account_balances(owner_id);

-- Transaction history projection
CREATE TABLE IF NOT EXISTS transactions (
    id         UUID        PRIMARY KEY DEFAULT uuid_generate_v4(),
    account_id UUID        NOT NULL,
    event_type VARCHAR(50) NOT NULL,
    amount     BIGINT      NOT NULL,
    currency   CHAR(3)     NOT NULL,
    direction  VARCHAR(6)  NOT NULL CHECK (direction IN ('CREDIT','DEBIT')),
    reference  TEXT        NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_transactions_account   ON transactions(account_id);
CREATE INDEX IF NOT EXISTS idx_transactions_created   ON transactions(created_at);
CREATE INDEX IF NOT EXISTS idx_transactions_direction ON transactions(direction);

