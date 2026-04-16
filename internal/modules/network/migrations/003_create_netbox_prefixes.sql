-- +goose Up
CREATE TABLE netbox_prefixes (
    id         TEXT PRIMARY KEY,
    netbox_id  INTEGER NOT NULL UNIQUE,
    cidr       TEXT NOT NULL DEFAULT '',
    location   TEXT NOT NULL,
    priority   INTEGER NOT NULL DEFAULT 0,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX idx_netbox_prefixes_location ON netbox_prefixes(location, priority);

-- +goose Down
DROP TABLE IF EXISTS netbox_prefixes;
