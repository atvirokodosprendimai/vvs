-- +goose Up
CREATE TABLE IF NOT EXISTS iptv_epg_programmes (
    id            TEXT PRIMARY KEY,
    channel_epg_id TEXT NOT NULL,
    title          TEXT NOT NULL DEFAULT '',
    description    TEXT NOT NULL DEFAULT '',
    start_time     INTEGER NOT NULL,
    stop_time      INTEGER NOT NULL,
    category       TEXT NOT NULL DEFAULT '',
    rating         TEXT NOT NULL DEFAULT '',
    UNIQUE (channel_epg_id, start_time)
);

CREATE INDEX IF NOT EXISTS idx_epg_channel_start ON iptv_epg_programmes (channel_epg_id, start_time);
CREATE INDEX IF NOT EXISTS idx_epg_stop ON iptv_epg_programmes (stop_time);

-- +goose Down
DROP INDEX IF EXISTS idx_epg_stop;
DROP INDEX IF EXISTS idx_epg_channel_start;
DROP TABLE IF EXISTS iptv_epg_programmes;
