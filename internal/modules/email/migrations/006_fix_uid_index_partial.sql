-- +goose Up
-- Restore partial unique index so outgoing (direction='out') messages are not constrained.
-- Migration 005 accidentally dropped the WHERE direction='in' partial filter.
DROP INDEX IF EXISTS idx_email_messages_uid;
CREATE UNIQUE INDEX idx_email_messages_uid ON email_messages(account_id, folder, uid) WHERE direction = 'in';

-- +goose Down
DROP INDEX IF EXISTS idx_email_messages_uid;
CREATE UNIQUE INDEX idx_email_messages_uid ON email_messages(account_id, folder, uid);
