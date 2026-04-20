-- +goose Up
CREATE TABLE IF NOT EXISTS swarm_nodes (
    id              TEXT PRIMARY KEY,
    cluster_id      TEXT NOT NULL DEFAULT '',
    role            TEXT NOT NULL DEFAULT 'worker',
    name            TEXT NOT NULL,
    ssh_host        TEXT NOT NULL,
    ssh_user        TEXT NOT NULL DEFAULT 'root',
    ssh_port        INTEGER NOT NULL DEFAULT 22,
    ssh_key         BLOB NOT NULL DEFAULT '',
    vpn_ip          TEXT NOT NULL DEFAULT '',
    swarm_node_id   TEXT NOT NULL DEFAULT '',
    status          TEXT NOT NULL DEFAULT 'provisioning',
    created_at      DATETIME NOT NULL,
    updated_at      DATETIME NOT NULL
);

-- +goose Down
DROP TABLE IF EXISTS swarm_nodes;
