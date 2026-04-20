-- +goose Up
CREATE TABLE IF NOT EXISTS swarm_clusters (
    id              TEXT PRIMARY KEY,
    name            TEXT NOT NULL,
    wgmesh_key      BLOB NOT NULL DEFAULT '',
    manager_token   BLOB NOT NULL DEFAULT '',
    worker_token    BLOB NOT NULL DEFAULT '',
    advertise_addr  TEXT NOT NULL DEFAULT '',
    notes           TEXT NOT NULL DEFAULT '',
    status          TEXT NOT NULL DEFAULT 'initializing',
    created_at      DATETIME NOT NULL,
    updated_at      DATETIME NOT NULL
);

-- +goose Down
DROP TABLE IF EXISTS swarm_clusters;
