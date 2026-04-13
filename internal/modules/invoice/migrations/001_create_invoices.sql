-- +goose Up
CREATE TABLE IF NOT EXISTS invoices (
    id TEXT PRIMARY KEY,
    invoice_number TEXT NOT NULL UNIQUE,
    customer_id TEXT NOT NULL,
    customer_name TEXT NOT NULL,
    subtotal_amount INTEGER NOT NULL DEFAULT 0,
    subtotal_currency TEXT NOT NULL DEFAULT 'EUR',
    tax_rate INTEGER NOT NULL DEFAULT 21,
    tax_amount INTEGER NOT NULL DEFAULT 0,
    tax_currency TEXT NOT NULL DEFAULT 'EUR',
    total_amount INTEGER NOT NULL DEFAULT 0,
    total_currency TEXT NOT NULL DEFAULT 'EUR',
    status TEXT NOT NULL DEFAULT 'draft',
    issue_date DATETIME NOT NULL,
    due_date DATETIME NOT NULL,
    paid_date DATETIME,
    recurring_id TEXT,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS invoice_lines (
    id TEXT PRIMARY KEY,
    invoice_id TEXT NOT NULL,
    product_id TEXT DEFAULT '',
    product_name TEXT DEFAULT '',
    description TEXT DEFAULT '',
    quantity INTEGER NOT NULL DEFAULT 1,
    unit_price_amount INTEGER NOT NULL DEFAULT 0,
    unit_price_currency TEXT NOT NULL DEFAULT 'EUR',
    total_amount INTEGER NOT NULL DEFAULT 0,
    total_currency TEXT NOT NULL DEFAULT 'EUR',
    sort_order INTEGER NOT NULL DEFAULT 0,
    FOREIGN KEY (invoice_id) REFERENCES invoices(id) ON DELETE CASCADE
);

CREATE INDEX idx_invoices_number ON invoices(invoice_number);
CREATE INDEX idx_invoices_customer ON invoices(customer_id);
CREATE INDEX idx_invoices_status ON invoices(status);
CREATE INDEX idx_invoice_lines_invoice ON invoice_lines(invoice_id);

CREATE TABLE IF NOT EXISTS invoice_number_sequences (
    year INTEGER PRIMARY KEY,
    last_number INTEGER NOT NULL DEFAULT 0
);

-- +goose Down
DROP TABLE IF EXISTS invoice_number_sequences;
DROP TABLE IF EXISTS invoice_lines;
DROP TABLE IF EXISTS invoices;
