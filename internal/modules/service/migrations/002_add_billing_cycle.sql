-- +goose Up
ALTER TABLE customer_services ADD COLUMN billing_cycle TEXT NOT NULL DEFAULT 'monthly';
ALTER TABLE customer_services ADD COLUMN next_billing_date DATETIME;

-- +goose Down
-- SQLite pre-3.35 has no DROP COLUMN; columns remain
