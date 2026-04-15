-- +goose Up
ALTER TABLE customers ADD COLUMN router_id TEXT DEFAULT NULL;
ALTER TABLE customers ADD COLUMN ip_address TEXT NOT NULL DEFAULT '';
ALTER TABLE customers ADD COLUMN mac_address TEXT NOT NULL DEFAULT '';

CREATE INDEX idx_customers_router_id ON customers(router_id);

-- +goose Down
DROP INDEX IF EXISTS idx_customers_router_id;
-- SQLite does not support DROP COLUMN before 3.35; columns are left in place on rollback
