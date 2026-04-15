-- +goose Up
ALTER TABLE routers ADD COLUMN router_type TEXT NOT NULL DEFAULT 'mikrotik';

-- +goose Down
-- SQLite does not support DROP COLUMN on older versions; leave column in place.
