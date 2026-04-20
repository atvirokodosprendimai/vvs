-- +goose Up
ALTER TABLE swarm_stacks ADD COLUMN target_node_id TEXT NOT NULL DEFAULT '';

-- +goose Down
-- SQLite does not support DROP COLUMN on older versions; intentionally irreversible.
