-- +goose Up

CREATE TABLE email_account_folders (
    id         TEXT PRIMARY KEY,
    account_id TEXT NOT NULL REFERENCES email_accounts(id) ON DELETE CASCADE,
    name       TEXT NOT NULL,
    last_uid   INTEGER NOT NULL DEFAULT 0,
    enabled    INTEGER NOT NULL DEFAULT 1,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(account_id, name)
);
CREATE INDEX idx_email_folders_account ON email_account_folders(account_id);

-- Seed existing accounts: migrate their single folder + last_uid into the new table.
-- Each account gets one row for its configured folder.
INSERT OR IGNORE INTO email_account_folders (id, account_id, name, last_uid, enabled, created_at)
SELECT lower(hex(randomblob(4)) || '-' || hex(randomblob(2)) || '-' || hex(randomblob(2)) || '-' || hex(randomblob(2)) || '-' || hex(randomblob(6))),
       id, folder, last_uid, 1, CURRENT_TIMESTAMP
FROM email_accounts;

-- +goose Down
DROP TABLE IF EXISTS email_account_folders;
