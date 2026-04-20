-- +goose Up
ALTER TABLE iptv_stacks ADD COLUMN wan_interface TEXT NOT NULL DEFAULT '';

-- +goose Down
SELECT 1; -- SQLite cannot drop columns; no-op
