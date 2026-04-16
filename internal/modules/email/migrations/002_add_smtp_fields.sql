-- +goose Up
ALTER TABLE email_accounts ADD COLUMN smtp_host TEXT NOT NULL DEFAULT '';
ALTER TABLE email_accounts ADD COLUMN smtp_port INTEGER NOT NULL DEFAULT 587;
ALTER TABLE email_accounts ADD COLUMN smtp_tls  TEXT NOT NULL DEFAULT 'starttls';

-- +goose Down
