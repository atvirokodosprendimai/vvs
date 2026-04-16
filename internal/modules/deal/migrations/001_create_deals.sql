-- +goose Up
CREATE TABLE deals (
    id          TEXT PRIMARY KEY,
    customer_id TEXT NOT NULL REFERENCES customers(id) ON DELETE CASCADE,
    title       TEXT NOT NULL,
    value       INTEGER NOT NULL DEFAULT 0,
    currency    TEXT NOT NULL DEFAULT 'EUR',
    stage       TEXT NOT NULL DEFAULT 'new'
                CHECK(stage IN ('new','qualified','proposal','negotiation','won','lost')),
    notes       TEXT NOT NULL DEFAULT '',
    created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX idx_deals_customer ON deals(customer_id);

-- +goose Down
DROP TABLE IF EXISTS deals;
