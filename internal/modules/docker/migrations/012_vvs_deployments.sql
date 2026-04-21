-- +goose Up
CREATE TABLE IF NOT EXISTS vvs_deployments (
    id               TEXT PRIMARY KEY,
    cluster_id       TEXT NOT NULL DEFAULT '',
    node_id          TEXT NOT NULL DEFAULT '',
    component        TEXT NOT NULL DEFAULT '',
    source           TEXT NOT NULL DEFAULT 'image',
    image_url        TEXT NOT NULL DEFAULT '',
    registry_id      TEXT NOT NULL DEFAULT '',
    git_url          TEXT NOT NULL DEFAULT '',
    git_ref          TEXT NOT NULL DEFAULT 'main',
    nats_url         TEXT NOT NULL DEFAULT '',
    port             INTEGER NOT NULL DEFAULT 8080,
    env_vars         TEXT NOT NULL DEFAULT '{}',
    status           TEXT NOT NULL DEFAULT 'pending',
    error_msg        TEXT NOT NULL DEFAULT '',
    last_deployed_at DATETIME,
    created_at       DATETIME NOT NULL,
    updated_at       DATETIME NOT NULL
);

-- +goose Down
DROP TABLE IF EXISTS vvs_deployments;
