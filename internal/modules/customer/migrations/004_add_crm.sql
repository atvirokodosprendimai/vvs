-- +goose Up
CREATE TABLE IF NOT EXISTS customer_notes (
    id          TEXT PRIMARY KEY,
    customer_id TEXT NOT NULL REFERENCES customers(id) ON DELETE CASCADE,
    body        TEXT NOT NULL,
    author_id   TEXT NOT NULL DEFAULT '',
    created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_customer_notes_customer ON customer_notes(customer_id, created_at DESC);

-- +goose Down
DROP TABLE IF EXISTS customer_notes;
