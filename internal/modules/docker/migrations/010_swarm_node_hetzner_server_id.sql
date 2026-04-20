-- +goose Up
ALTER TABLE swarm_nodes ADD COLUMN hetzner_server_id INTEGER NOT NULL DEFAULT 0;

-- +goose Down
-- SQLite does not support DROP COLUMN in older versions; leave as-is
