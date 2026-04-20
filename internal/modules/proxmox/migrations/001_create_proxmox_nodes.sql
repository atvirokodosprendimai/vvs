-- +goose Up
CREATE TABLE proxmox_nodes (
    id           TEXT PRIMARY KEY,
    name         TEXT NOT NULL,
    node_name    TEXT NOT NULL,
    host         TEXT NOT NULL,
    port         INTEGER NOT NULL DEFAULT 8006,
    "user"       TEXT NOT NULL,
    token_id     TEXT NOT NULL,
    token_secret TEXT NOT NULL,
    insecure_tls INTEGER NOT NULL DEFAULT 0,
    notes        TEXT NOT NULL DEFAULT '',
    created_at   DATETIME NOT NULL,
    updated_at   DATETIME NOT NULL
);

-- +goose Down
DROP TABLE proxmox_nodes;
