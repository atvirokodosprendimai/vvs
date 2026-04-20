-- +goose Up
CREATE TABLE IF NOT EXISTS iptv_channel_providers (
    id          TEXT NOT NULL PRIMARY KEY,
    channel_id  TEXT NOT NULL REFERENCES iptv_channels(id) ON DELETE CASCADE,
    name        TEXT NOT NULL DEFAULT '',
    url_template TEXT NOT NULL DEFAULT '',
    token       TEXT NOT NULL DEFAULT '',
    type        TEXT NOT NULL DEFAULT 'external',
    priority    INTEGER NOT NULL DEFAULT 0,
    active      BOOLEAN NOT NULL DEFAULT 1,
    created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_iptv_channel_providers_channel_id ON iptv_channel_providers(channel_id);

-- +goose Down
DROP TABLE IF EXISTS iptv_channel_providers;
