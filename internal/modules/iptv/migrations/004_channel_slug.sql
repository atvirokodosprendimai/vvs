-- +goose Up
ALTER TABLE iptv_channels ADD COLUMN slug TEXT NOT NULL DEFAULT '';

-- +goose Down
-- SQLite does not support DROP COLUMN in older versions; migration is irreversible
