-- +goose Up
CREATE TABLE notifications (
    id         TEXT     PRIMARY KEY,
    title      TEXT     NOT NULL,
    url        TEXT     NOT NULL DEFAULT '',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- per-user read tracking (insert-only; no row = unread)
CREATE TABLE notification_reads (
    user_id         TEXT NOT NULL,
    notification_id TEXT NOT NULL,
    PRIMARY KEY (user_id, notification_id)
);

-- +goose Down
DROP TABLE IF EXISTS notification_reads;
DROP TABLE IF EXISTS notifications;
