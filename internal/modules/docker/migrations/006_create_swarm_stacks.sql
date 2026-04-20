-- +goose Up
CREATE TABLE IF NOT EXISTS swarm_stacks (
    id           TEXT PRIMARY KEY,
    cluster_id   TEXT NOT NULL,
    name         TEXT NOT NULL,
    compose_yaml TEXT NOT NULL DEFAULT '',
    status       TEXT NOT NULL DEFAULT 'deploying',
    error_msg    TEXT NOT NULL DEFAULT '',
    created_at   DATETIME NOT NULL,
    updated_at   DATETIME NOT NULL
);

-- +goose Down
DROP TABLE IF EXISTS swarm_stacks;
