-- +goose Up
CREATE TABLE portal_tokens (
    id          TEXT PRIMARY KEY,
    customer_id TEXT NOT NULL REFERENCES customers(id) ON DELETE CASCADE,
    token_hash  TEXT NOT NULL UNIQUE,
    expires_at  DATETIME NOT NULL,
    created_at  DATETIME NOT NULL
);
CREATE INDEX idx_portal_tokens_customer_id ON portal_tokens(customer_id);

-- +goose Down
DROP TABLE IF EXISTS portal_tokens;
