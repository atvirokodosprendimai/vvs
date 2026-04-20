-- +goose Up
ALTER TABLE swarm_clusters ADD COLUMN hetzner_api_key  BLOB    NOT NULL DEFAULT '';
ALTER TABLE swarm_clusters ADD COLUMN hetzner_ssh_key_id INTEGER NOT NULL DEFAULT 0;
ALTER TABLE swarm_clusters ADD COLUMN ssh_private_key  BLOB    NOT NULL DEFAULT '';
ALTER TABLE swarm_clusters ADD COLUMN ssh_public_key   TEXT    NOT NULL DEFAULT '';

-- +goose Down
-- SQLite does not support DROP COLUMN on older versions; migration is intentionally irreversible.
