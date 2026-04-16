-- +goose Up
ALTER TABLE customers ADD COLUMN network_zone TEXT NOT NULL DEFAULT '';

-- +goose Down
-- SQLite cannot drop columns in older versions; leave as-is
