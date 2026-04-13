-- +goose Up
CREATE TABLE IF NOT EXISTS recurring_invoices (
    id TEXT PRIMARY KEY,
    customer_id TEXT NOT NULL,
    customer_name TEXT NOT NULL,
    frequency TEXT NOT NULL DEFAULT 'monthly',
    day_of_month INTEGER NOT NULL DEFAULT 1,
    next_run_date DATETIME NOT NULL,
    last_run_date DATETIME,
    status TEXT NOT NULL DEFAULT 'active',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS recurring_lines (
    id TEXT PRIMARY KEY,
    recurring_id TEXT NOT NULL,
    product_id TEXT DEFAULT '',
    product_name TEXT DEFAULT '',
    description TEXT DEFAULT '',
    quantity INTEGER NOT NULL DEFAULT 1,
    unit_price_amount INTEGER NOT NULL DEFAULT 0,
    unit_price_currency TEXT NOT NULL DEFAULT 'EUR',
    sort_order INTEGER NOT NULL DEFAULT 0,
    FOREIGN KEY (recurring_id) REFERENCES recurring_invoices(id) ON DELETE CASCADE
);

CREATE INDEX idx_recurring_customer ON recurring_invoices(customer_id);
CREATE INDEX idx_recurring_status ON recurring_invoices(status);
CREATE INDEX idx_recurring_next_run ON recurring_invoices(next_run_date);
CREATE INDEX idx_recurring_lines_recurring ON recurring_lines(recurring_id);

-- +goose Down
DROP TABLE IF EXISTS recurring_lines;
DROP TABLE IF EXISTS recurring_invoices;
