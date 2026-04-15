-- +goose Up
CREATE TABLE chat_messages (
    id         TEXT     PRIMARY KEY,
    user_id    TEXT     NOT NULL,
    username   TEXT     NOT NULL,
    body       TEXT     NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- +goose Down
DROP TABLE IF EXISTS chat_messages;
