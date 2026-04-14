-- +goose Up
CREATE TABLE IF NOT EXISTS debt_statuses (
    id TEXT PRIMARY KEY,
    customer_id TEXT NOT NULL UNIQUE,
    tax_id TEXT NOT NULL DEFAULT '',
    over_credit_budget INTEGER NOT NULL DEFAULT 0,
    synced_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (customer_id) REFERENCES customers(id) ON DELETE CASCADE
);

CREATE INDEX idx_debt_statuses_customer_id ON debt_statuses(customer_id);
CREATE INDEX idx_debt_statuses_tax_id ON debt_statuses(tax_id);
CREATE INDEX idx_debt_statuses_over_credit_budget ON debt_statuses(over_credit_budget);

-- +goose Down
DROP TABLE IF EXISTS debt_statuses;
