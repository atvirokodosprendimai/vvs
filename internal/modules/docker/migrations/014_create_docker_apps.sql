-- +goose Up
CREATE TABLE IF NOT EXISTS docker_apps (
    id             TEXT PRIMARY KEY,
    name           TEXT NOT NULL DEFAULT '',
    repo_url       TEXT NOT NULL DEFAULT '',
    branch         TEXT NOT NULL DEFAULT 'main',
    reg_user       TEXT NOT NULL DEFAULT '',
    reg_pass       BLOB,
    build_args     TEXT NOT NULL DEFAULT '[]',
    env_vars       TEXT NOT NULL DEFAULT '[]',
    ports          TEXT NOT NULL DEFAULT '[]',
    volumes        TEXT NOT NULL DEFAULT '[]',
    networks       TEXT NOT NULL DEFAULT '[]',
    restart_policy TEXT NOT NULL DEFAULT 'unless-stopped',
    container_name TEXT NOT NULL DEFAULT '',
    image_ref      TEXT NOT NULL DEFAULT '',
    status         TEXT NOT NULL DEFAULT 'idle',
    error_msg      TEXT NOT NULL DEFAULT '',
    last_built_at  DATETIME,
    created_at     DATETIME NOT NULL,
    updated_at     DATETIME NOT NULL
);

-- +goose Down
DROP TABLE IF EXISTS docker_apps;
