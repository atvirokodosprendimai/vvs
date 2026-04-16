-- +goose Up
ALTER TABLE email_messages ADD COLUMN direction TEXT NOT NULL DEFAULT 'in';
DROP INDEX IF EXISTS idx_email_messages_uid;
CREATE UNIQUE INDEX idx_email_messages_uid ON email_messages(account_id, uid) WHERE direction = 'in';

-- +goose Down
