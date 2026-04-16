-- Fix: IMAP UIDs are per-folder, not per-account.
-- The old (account_id, uid) unique index allows UID collisions across folders.
DROP INDEX IF EXISTS idx_email_messages_uid;
CREATE UNIQUE INDEX idx_email_messages_uid ON email_messages(account_id, folder, uid);
