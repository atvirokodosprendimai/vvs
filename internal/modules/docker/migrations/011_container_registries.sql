-- +goose Up
CREATE TABLE IF NOT EXISTS container_registries (
    id          TEXT PRIMARY KEY,
    name        TEXT NOT NULL,
    url         TEXT NOT NULL DEFAULT '',
    username    TEXT NOT NULL DEFAULT '',
    password    BLOB,
    created_at  DATETIME NOT NULL,
    updated_at  DATETIME NOT NULL
);

-- +goose Down
DROP TABLE IF EXISTS container_registries;
