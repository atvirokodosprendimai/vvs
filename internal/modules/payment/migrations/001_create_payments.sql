-- +goose Up
CREATE TABLE IF NOT EXISTS payments (
    id TEXT PRIMARY KEY,
    amount_cents INTEGER NOT NULL DEFAULT 0,
    amount_currency TEXT NOT NULL DEFAULT 'EUR',
    reference TEXT DEFAULT '',
    payer_name TEXT DEFAULT '',
    payer_iban TEXT DEFAULT '',
    booking_date DATETIME NOT NULL,
    invoice_id TEXT,
    customer_id TEXT,
    status TEXT NOT NULL DEFAULT 'imported',
    import_batch_id TEXT DEFAULT '',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_payments_status ON payments(status);
CREATE INDEX idx_payments_invoice ON payments(invoice_id);
CREATE INDEX idx_payments_customer ON payments(customer_id);
CREATE INDEX idx_payments_batch ON payments(import_batch_id);
CREATE INDEX idx_payments_booking ON payments(booking_date);

-- +goose Down
DROP TABLE IF EXISTS payments;
