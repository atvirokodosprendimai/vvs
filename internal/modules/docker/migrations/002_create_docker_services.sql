-- +goose Up
CREATE TABLE docker_services (
    id           TEXT PRIMARY KEY,
    node_id      TEXT NOT NULL REFERENCES docker_nodes(id),
    name         TEXT NOT NULL,
    compose_yaml TEXT NOT NULL DEFAULT '',
    status       TEXT NOT NULL DEFAULT 'stopped',
    error_msg    TEXT NOT NULL DEFAULT '',
    created_at   DATETIME NOT NULL,
    updated_at   DATETIME NOT NULL
);

CREATE INDEX idx_docker_services_node_id ON docker_services(node_id);

-- +goose Down
DROP INDEX IF EXISTS idx_docker_services_node_id;
DROP TABLE docker_services;
