-- +goose Up
CREATE TABLE invoice_tokens (
    id         TEXT PRIMARY KEY,
    invoice_id TEXT NOT NULL REFERENCES invoices(id) ON DELETE CASCADE,
    token_hash TEXT NOT NULL UNIQUE,
    expires_at DATETIME NOT NULL,
    created_at DATETIME NOT NULL
);

CREATE INDEX idx_invoice_tokens_invoice_id ON invoice_tokens(invoice_id);

-- +goose Down
DROP INDEX IF EXISTS idx_invoice_tokens_invoice_id;
DROP TABLE IF EXISTS invoice_tokens;
