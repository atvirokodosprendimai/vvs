-- +goose Up
ALTER TABLE email_accounts ADD COLUMN sent_folder TEXT NOT NULL DEFAULT 'Sent';

-- +goose Down
-- SQLite does not support DROP COLUMN in older versions; no-op is safe here.
