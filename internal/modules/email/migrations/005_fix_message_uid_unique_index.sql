-- +goose Up
-- Fix: IMAP UIDs are per-folder, not per-account.
-- Remove duplicate (account_id, folder, uid) rows before creating the new index.
DELETE FROM email_messages
WHERE rowid NOT IN (
    SELECT MIN(rowid)
    FROM email_messages
    GROUP BY account_id, folder, uid
);
DROP INDEX IF EXISTS idx_email_messages_uid;
CREATE UNIQUE INDEX idx_email_messages_uid ON email_messages(account_id, folder, uid);

-- +goose Down
DROP INDEX IF EXISTS idx_email_messages_uid;
CREATE UNIQUE INDEX idx_email_messages_uid ON email_messages(account_id, uid);
