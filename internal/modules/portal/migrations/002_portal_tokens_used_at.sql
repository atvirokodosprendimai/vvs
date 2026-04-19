-- +goose Up
ALTER TABLE portal_tokens ADD COLUMN used_at DATETIME NULL;

-- +goose Down
-- SQLite does not support DROP COLUMN in older versions; recreate table.
CREATE TABLE portal_tokens_new (
    id          TEXT PRIMARY KEY,
    customer_id TEXT NOT NULL REFERENCES customers(id) ON DELETE CASCADE,
    token_hash  TEXT NOT NULL UNIQUE,
    expires_at  DATETIME NOT NULL,
    created_at  DATETIME NOT NULL
);
INSERT INTO portal_tokens_new SELECT id, customer_id, token_hash, expires_at, created_at FROM portal_tokens;
DROP TABLE portal_tokens;
ALTER TABLE portal_tokens_new RENAME TO portal_tokens;
CREATE INDEX idx_portal_tokens_customer_id ON portal_tokens(customer_id);
