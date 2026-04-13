-- +goose Up
CREATE TABLE IF NOT EXISTS products (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    description TEXT DEFAULT '',
    type TEXT NOT NULL DEFAULT 'internet',
    price_amount INTEGER NOT NULL DEFAULT 0,
    price_currency TEXT NOT NULL DEFAULT 'EUR',
    billing_period TEXT NOT NULL DEFAULT 'monthly',
    is_active INTEGER NOT NULL DEFAULT 1,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_products_type ON products(type);
CREATE INDEX idx_products_active ON products(is_active);

-- +goose Down
DROP TABLE IF EXISTS products;
