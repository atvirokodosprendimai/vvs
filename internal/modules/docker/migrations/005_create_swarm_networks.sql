-- +goose Up
CREATE TABLE IF NOT EXISTS swarm_networks (
    id                TEXT PRIMARY KEY,
    cluster_id        TEXT NOT NULL DEFAULT '',
    name              TEXT NOT NULL,
    driver            TEXT NOT NULL DEFAULT 'overlay',
    subnet            TEXT NOT NULL DEFAULT '',
    gateway           TEXT NOT NULL DEFAULT '',
    dhcp_range_start  TEXT NOT NULL DEFAULT '',
    dhcp_range_end    TEXT NOT NULL DEFAULT '',
    parent            TEXT NOT NULL DEFAULT '',
    options           TEXT NOT NULL DEFAULT '{}',
    reserved_ips      TEXT NOT NULL DEFAULT '[]',
    scope             TEXT NOT NULL DEFAULT 'swarm',
    docker_network_id TEXT NOT NULL DEFAULT '',
    created_at        DATETIME NOT NULL,
    updated_at        DATETIME NOT NULL
);

-- +goose Down
DROP TABLE IF EXISTS swarm_networks;
