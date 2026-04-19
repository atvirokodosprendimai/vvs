-- +goose Up
ALTER TABLE invoices ADD COLUMN reminder_sent_at DATETIME;

-- +goose Down
-- SQLite does not support DROP COLUMN in older versions; leave column in place.
