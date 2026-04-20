-- +goose Up
CREATE TABLE IF NOT EXISTS swarm_routes (
    id           TEXT PRIMARY KEY,
    stack_id     TEXT NOT NULL,
    hostname     TEXT NOT NULL,
    port         INTEGER NOT NULL DEFAULT 80,
    strip_prefix INTEGER NOT NULL DEFAULT 0,
    created_at   DATETIME NOT NULL
);

-- +goose Down
DROP TABLE IF EXISTS swarm_routes;
