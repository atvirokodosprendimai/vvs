-- +goose Up
CREATE TABLE IF NOT EXISTS customer_balance (
    customer_id  TEXT PRIMARY KEY,
    balance_cents INTEGER NOT NULL DEFAULT 0,
    updated_at   DATETIME NOT NULL
);

CREATE TABLE IF NOT EXISTS balance_ledger (
    id                TEXT PRIMARY KEY,
    customer_id       TEXT NOT NULL,
    type              TEXT NOT NULL,
    amount_cents      INTEGER NOT NULL,
    description       TEXT NOT NULL DEFAULT '',
    stripe_session_id TEXT NOT NULL DEFAULT '',
    created_at        DATETIME NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_balance_ledger_customer ON balance_ledger(customer_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_balance_ledger_stripe ON balance_ledger(stripe_session_id)
    WHERE stripe_session_id != '';

-- +goose Down
DROP TABLE IF EXISTS balance_ledger;
DROP TABLE IF EXISTS customer_balance;
