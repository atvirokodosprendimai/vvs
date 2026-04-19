-- +goose Up

CREATE TABLE iptv_channels (
    id         TEXT PRIMARY KEY,
    name       TEXT NOT NULL,
    logo_url   TEXT NOT NULL DEFAULT '',
    stream_url TEXT NOT NULL DEFAULT '',
    category   TEXT NOT NULL DEFAULT '',
    epg_source TEXT NOT NULL DEFAULT '',
    active     INTEGER NOT NULL DEFAULT 1,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX idx_iptv_channels_category ON iptv_channels(category);

CREATE TABLE iptv_packages (
    id           TEXT PRIMARY KEY,
    name         TEXT NOT NULL,
    price_cents  INTEGER NOT NULL DEFAULT 0,
    description  TEXT NOT NULL DEFAULT '',
    created_at   DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at   DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE iptv_package_channels (
    package_id TEXT NOT NULL REFERENCES iptv_packages(id) ON DELETE CASCADE,
    channel_id TEXT NOT NULL REFERENCES iptv_channels(id) ON DELETE CASCADE,
    PRIMARY KEY (package_id, channel_id)
);

CREATE TABLE iptv_subscriptions (
    id          TEXT PRIMARY KEY,
    customer_id TEXT NOT NULL,
    package_id  TEXT NOT NULL REFERENCES iptv_packages(id),
    status      TEXT NOT NULL DEFAULT 'active'
                CHECK(status IN ('active','suspended','cancelled')),
    starts_at   DATETIME NOT NULL,
    ends_at     DATETIME,
    created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX idx_iptv_subscriptions_customer ON iptv_subscriptions(customer_id);
CREATE INDEX idx_iptv_subscriptions_status   ON iptv_subscriptions(status);

CREATE TABLE iptv_stbs (
    id          TEXT PRIMARY KEY,
    mac         TEXT NOT NULL UNIQUE,
    model       TEXT NOT NULL DEFAULT '',
    firmware    TEXT NOT NULL DEFAULT '',
    serial      TEXT NOT NULL DEFAULT '',
    customer_id TEXT NOT NULL,
    assigned_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    notes       TEXT NOT NULL DEFAULT '',
    created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX idx_iptv_stbs_customer ON iptv_stbs(customer_id);

CREATE TABLE iptv_subscription_keys (
    id              TEXT PRIMARY KEY,
    subscription_id TEXT NOT NULL REFERENCES iptv_subscriptions(id),
    customer_id     TEXT NOT NULL,
    package_id      TEXT NOT NULL,
    token           TEXT NOT NULL UNIQUE,
    created_at      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    revoked_at      DATETIME
);
CREATE INDEX idx_iptv_keys_subscription ON iptv_subscription_keys(subscription_id);
CREATE INDEX idx_iptv_keys_token        ON iptv_subscription_keys(token);

-- +goose Down

DROP TABLE IF EXISTS iptv_subscription_keys;
DROP TABLE IF EXISTS iptv_stbs;
DROP TABLE IF EXISTS iptv_subscriptions;
DROP TABLE IF EXISTS iptv_package_channels;
DROP TABLE IF EXISTS iptv_packages;
DROP TABLE IF EXISTS iptv_channels;
