-- +goose Up
CREATE TABLE chat_threads (
    id         TEXT PRIMARY KEY,
    type       TEXT NOT NULL CHECK(type IN ('direct', 'channel')),
    name       TEXT,
    is_private INTEGER NOT NULL DEFAULT 0,
    created_by TEXT NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE chat_thread_members (
    thread_id TEXT NOT NULL REFERENCES chat_threads(id) ON DELETE CASCADE,
    user_id   TEXT NOT NULL,
    joined_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (thread_id, user_id)
);

CREATE TABLE chat_thread_reads (
    thread_id    TEXT NOT NULL,
    user_id      TEXT NOT NULL,
    last_read_at DATETIME NOT NULL,
    PRIMARY KEY (thread_id, user_id)
);

ALTER TABLE chat_messages ADD COLUMN thread_id TEXT NOT NULL DEFAULT '';

-- Seed #general channel
INSERT INTO chat_threads (id, type, name, is_private, created_by, created_at)
VALUES ('general', 'channel', '#general', 0, 'system', CURRENT_TIMESTAMP);

-- Backfill existing messages to #general
UPDATE chat_messages SET thread_id = 'general' WHERE thread_id = '';

-- +goose Down
DROP TABLE IF EXISTS chat_thread_reads;
DROP TABLE IF EXISTS chat_thread_members;
DROP TABLE IF EXISTS chat_threads;
