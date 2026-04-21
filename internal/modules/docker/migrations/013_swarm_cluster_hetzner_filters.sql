-- +goose Up
ALTER TABLE swarm_clusters ADD COLUMN hetzner_enabled_locations    TEXT NOT NULL DEFAULT '';
ALTER TABLE swarm_clusters ADD COLUMN hetzner_enabled_server_types TEXT NOT NULL DEFAULT '';

-- +goose Down
SELECT 1; -- SQLite does not support DROP COLUMN in older versions; leave as-is
