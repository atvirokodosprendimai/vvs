-- +goose Up
ALTER TABLE swarm_stacks ADD COLUMN registry_id TEXT NOT NULL DEFAULT '';

-- +goose Down
-- SQLite does not support DROP COLUMN reliably; leave column in place
