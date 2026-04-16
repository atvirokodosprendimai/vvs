-- +goose Up
CREATE TABLE email_accounts (
    id           TEXT PRIMARY KEY,
    name         TEXT NOT NULL,
    host         TEXT NOT NULL,
    port         INTEGER NOT NULL DEFAULT 993,
    username     TEXT NOT NULL,
    password_enc BLOB NOT NULL DEFAULT '',
    tls          TEXT NOT NULL DEFAULT 'tls' CHECK(tls IN ('none','starttls','tls')),
    folder       TEXT NOT NULL DEFAULT 'INBOX',
    status       TEXT NOT NULL DEFAULT 'active' CHECK(status IN ('active','paused','error')),
    last_error   TEXT NOT NULL DEFAULT '',
    last_sync_at DATETIME,
    last_uid     INTEGER NOT NULL DEFAULT 0,
    created_at   DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at   DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE email_threads (
    id                    TEXT PRIMARY KEY,
    account_id            TEXT NOT NULL REFERENCES email_accounts(id) ON DELETE CASCADE,
    subject               TEXT NOT NULL DEFAULT '',
    participant_addresses TEXT NOT NULL DEFAULT '',
    customer_id           TEXT NOT NULL DEFAULT '',
    message_count         INTEGER NOT NULL DEFAULT 0,
    last_message_at       DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    created_at            DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX idx_email_threads_account     ON email_threads(account_id, last_message_at DESC);
CREATE INDEX idx_email_threads_customer    ON email_threads(customer_id) WHERE customer_id != '';

CREATE TABLE email_messages (
    id          TEXT PRIMARY KEY,
    account_id  TEXT NOT NULL REFERENCES email_accounts(id) ON DELETE CASCADE,
    thread_id   TEXT NOT NULL REFERENCES email_threads(id) ON DELETE CASCADE,
    uid         INTEGER NOT NULL DEFAULT 0,
    folder      TEXT NOT NULL DEFAULT 'INBOX',
    message_id  TEXT NOT NULL DEFAULT '',
    references  TEXT NOT NULL DEFAULT '',
    in_reply_to TEXT NOT NULL DEFAULT '',
    subject     TEXT NOT NULL DEFAULT '',
    from_addr   TEXT NOT NULL DEFAULT '',
    from_name   TEXT NOT NULL DEFAULT '',
    to_addrs    TEXT NOT NULL DEFAULT '',
    text_body   TEXT NOT NULL DEFAULT '',
    html_body   TEXT NOT NULL DEFAULT '',
    received_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    fetched_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE UNIQUE INDEX idx_email_messages_uid        ON email_messages(account_id, uid);
CREATE INDEX        idx_email_messages_thread     ON email_messages(thread_id, received_at ASC);
CREATE INDEX        idx_email_messages_message_id ON email_messages(account_id, message_id) WHERE message_id != '';

CREATE TABLE email_attachments (
    id         TEXT PRIMARY KEY,
    message_id TEXT NOT NULL REFERENCES email_messages(id) ON DELETE CASCADE,
    filename   TEXT NOT NULL DEFAULT '',
    mime_type  TEXT NOT NULL DEFAULT '',
    size       INTEGER NOT NULL DEFAULT 0,
    data       BLOB,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX idx_email_attachments_message ON email_attachments(message_id);

CREATE TABLE email_tags (
    id         TEXT PRIMARY KEY,
    account_id TEXT NOT NULL DEFAULT '',
    name       TEXT NOT NULL,
    color      TEXT NOT NULL DEFAULT '#6b7280',
    system     INTEGER NOT NULL DEFAULT 0,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE UNIQUE INDEX idx_email_tags_name ON email_tags(account_id, name);

CREATE TABLE email_thread_tags (
    thread_id TEXT NOT NULL REFERENCES email_threads(id) ON DELETE CASCADE,
    tag_id    TEXT NOT NULL REFERENCES email_tags(id) ON DELETE CASCADE,
    PRIMARY KEY (thread_id, tag_id)
);
CREATE INDEX idx_email_thread_tags_tag ON email_thread_tags(tag_id);

-- Seed system tags (global, account_id = '').
INSERT INTO email_tags (id, account_id, name, color, system) VALUES
    ('sys-unread',   '', 'unread',   '#3b82f6', 1),
    ('sys-starred',  '', 'starred',  '#f59e0b', 1),
    ('sys-archived', '', 'archived', '#6b7280', 1);

-- +goose Down
DROP TABLE IF EXISTS email_thread_tags;
DROP TABLE IF EXISTS email_tags;
DROP TABLE IF EXISTS email_attachments;
DROP TABLE IF EXISTS email_messages;
DROP TABLE IF EXISTS email_threads;
DROP TABLE IF EXISTS email_accounts;
