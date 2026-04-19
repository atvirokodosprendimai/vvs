-- +goose Up

ALTER TABLE iptv_channels ADD COLUMN dvr_url TEXT NOT NULL DEFAULT '';
ALTER TABLE iptv_package_channels ADD COLUMN position INTEGER NOT NULL DEFAULT 0;
CREATE INDEX idx_iptv_pkg_channels_pos ON iptv_package_channels(package_id, position);

-- +goose Down

DROP INDEX IF EXISTS idx_iptv_pkg_channels_pos;
-- SQLite does not support DROP COLUMN before 3.35; handled by recreating in future migrations if needed.
