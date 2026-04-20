-- +goose Up
CREATE TABLE docker_nodes (
    id         TEXT PRIMARY KEY,
    name       TEXT NOT NULL,
    host       TEXT NOT NULL,
    is_local   BOOLEAN NOT NULL DEFAULT 0,
    tls_cert   BLOB,
    tls_key    BLOB,
    tls_ca     BLOB,
    notes      TEXT NOT NULL DEFAULT '',
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL
);

-- +goose Down
DROP TABLE docker_nodes;
