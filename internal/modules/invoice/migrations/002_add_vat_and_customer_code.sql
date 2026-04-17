-- +goose Up

-- Legacy cleanup: drop old GORM-created tables with incompatible schemas (pre-goose era).
-- The old invoices table has completely different columns (invoice_number, tax_rate, etc.)
-- and cannot be ALTER'd into the new schema. Dev data only — safe to drop and recreate.
DROP TABLE IF EXISTS invoice_lines;
DROP TABLE IF EXISTS invoice_number_sequences;
-- Note: payments table belongs to payment module (goose_payment), do not drop it here.
-- recurring_invoices and recurring_lines belong to recurring module (goose_recurring).
DROP TABLE IF EXISTS recurring_lines;
DROP TABLE IF EXISTS recurring_invoices;
DROP TABLE IF EXISTS invoices;
DROP TABLE IF EXISTS invoice_line_items;
DROP TABLE IF EXISTS invoice_code_sequences;

-- Recreate canonical tables with full schema (base + VAT + customer_code).
CREATE TABLE invoices (
    id            TEXT PRIMARY KEY,
    customer_id   TEXT NOT NULL,
    customer_name TEXT NOT NULL DEFAULT '',
    customer_code TEXT NOT NULL DEFAULT '',
    code          TEXT NOT NULL UNIQUE,
    status        TEXT NOT NULL DEFAULT 'draft',
    issue_date    DATETIME NOT NULL,
    due_date      DATETIME NOT NULL,
    sub_total     INTEGER NOT NULL DEFAULT 0,
    vat_total     INTEGER NOT NULL DEFAULT 0,
    total_amount  INTEGER NOT NULL DEFAULT 0,
    currency      TEXT NOT NULL DEFAULT 'EUR',
    notes         TEXT NOT NULL DEFAULT '',
    paid_at       DATETIME,
    created_at    DATETIME NOT NULL,
    updated_at    DATETIME NOT NULL
);
CREATE INDEX idx_invoices_customer_id ON invoices(customer_id);
CREATE INDEX idx_invoices_status ON invoices(status);
CREATE INDEX idx_invoices_code ON invoices(code);

CREATE TABLE invoice_line_items (
    id              TEXT PRIMARY KEY,
    invoice_id      TEXT NOT NULL REFERENCES invoices(id) ON DELETE CASCADE,
    product_id      TEXT NOT NULL DEFAULT '',
    product_name    TEXT NOT NULL DEFAULT '',
    description     TEXT NOT NULL DEFAULT '',
    quantity        INTEGER NOT NULL DEFAULT 1,
    unit_price      INTEGER NOT NULL DEFAULT 0,
    unit_price_gross INTEGER NOT NULL DEFAULT 0,
    vat_rate        INTEGER NOT NULL DEFAULT 21,
    total_price     INTEGER NOT NULL DEFAULT 0,
    total_vat       INTEGER NOT NULL DEFAULT 0,
    total_gross     INTEGER NOT NULL DEFAULT 0
);
CREATE INDEX idx_invoice_line_items_invoice_id ON invoice_line_items(invoice_id);

CREATE TABLE invoice_code_sequences (
    prefix      TEXT PRIMARY KEY,
    last_number INTEGER NOT NULL DEFAULT 0
);

-- +goose Down
DROP TABLE IF EXISTS invoice_code_sequences;
DROP TABLE IF EXISTS invoice_line_items;
DROP TABLE IF EXISTS invoices;
