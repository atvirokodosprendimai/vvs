-- +goose Up
CREATE TABLE IF NOT EXISTS customers (
    id TEXT PRIMARY KEY,
    code TEXT NOT NULL UNIQUE,
    company_name TEXT NOT NULL,
    contact_name TEXT DEFAULT '',
    email TEXT DEFAULT '',
    phone TEXT DEFAULT '',
    street TEXT DEFAULT '',
    city TEXT DEFAULT '',
    postal_code TEXT DEFAULT '',
    country TEXT DEFAULT '',
    tax_id TEXT DEFAULT '',
    status TEXT NOT NULL DEFAULT 'active',
    notes TEXT DEFAULT '',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_customers_code ON customers(code);
CREATE INDEX idx_customers_status ON customers(status);
CREATE INDEX idx_customers_company_name ON customers(company_name);

CREATE TABLE IF NOT EXISTS company_code_sequences (
    prefix TEXT PRIMARY KEY,
    last_number INTEGER NOT NULL DEFAULT 0
);

INSERT OR IGNORE INTO company_code_sequences (prefix, last_number) VALUES ('CLI', 0);

-- +goose Down
DROP TABLE IF EXISTS company_code_sequences;
DROP TABLE IF EXISTS customers;
