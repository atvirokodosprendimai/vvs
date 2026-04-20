-- +goose Up
CREATE TABLE IF NOT EXISTS iptv_stacks (
    id                  TEXT NOT NULL PRIMARY KEY,
    name                TEXT NOT NULL DEFAULT '',
    cluster_id          TEXT NOT NULL DEFAULT '',
    node_id             TEXT NOT NULL DEFAULT '',
    wan_network_id      TEXT NOT NULL DEFAULT '',
    overlay_network_id  TEXT NOT NULL DEFAULT '',
    wan_network_name    TEXT NOT NULL DEFAULT '',
    overlay_network_name TEXT NOT NULL DEFAULT '',
    wan_ip              TEXT NOT NULL DEFAULT '',
    status              TEXT NOT NULL DEFAULT 'pending',
    last_deployed_at    DATETIME,
    created_at          DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at          DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS iptv_stack_channels (
    id          TEXT NOT NULL PRIMARY KEY,
    stack_id    TEXT NOT NULL REFERENCES iptv_stacks(id) ON DELETE CASCADE,
    channel_id  TEXT NOT NULL REFERENCES iptv_channels(id) ON DELETE CASCADE,
    provider_id TEXT NOT NULL DEFAULT '',
    UNIQUE(stack_id, channel_id)
);
CREATE INDEX IF NOT EXISTS idx_iptv_stack_channels_stack_id ON iptv_stack_channels(stack_id);

-- +goose Down
DROP TABLE IF EXISTS iptv_stack_channels;
DROP TABLE IF EXISTS iptv_stacks;
