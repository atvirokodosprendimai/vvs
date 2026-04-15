-- +goose Up
CREATE TABLE customer_services (
    id           TEXT PRIMARY KEY,
    customer_id  TEXT NOT NULL,
    product_id   TEXT NOT NULL,
    product_name TEXT NOT NULL,
    price_amount INTEGER NOT NULL,
    currency     TEXT NOT NULL DEFAULT 'EUR',
    start_date   DATETIME NOT NULL,
    status       TEXT NOT NULL DEFAULT 'active'
                 CHECK(status IN ('active','suspended','cancelled')),
    created_at   DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at   DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX idx_services_customer ON customer_services(customer_id);
CREATE INDEX idx_services_product  ON customer_services(product_id);

-- +goose Down
DROP TABLE IF EXISTS customer_services;
