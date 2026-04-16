-- +goose Up
CREATE TABLE devices (
    id              TEXT PRIMARY KEY,
    name            TEXT NOT NULL,
    serial_number   TEXT,
    device_type     TEXT NOT NULL DEFAULT 'other',
    status          TEXT NOT NULL DEFAULT 'in_stock'
                    CHECK(status IN ('in_stock', 'deployed', 'decommissioned')),
    customer_id     TEXT,
    location        TEXT,
    purchased_at    DATETIME,
    warranty_expiry DATETIME,
    notes           TEXT,
    created_at      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_devices_status   ON devices(status);
CREATE INDEX idx_devices_customer ON devices(customer_id);
CREATE UNIQUE INDEX idx_devices_serial ON devices(serial_number)
    WHERE serial_number IS NOT NULL AND serial_number != '';

-- +goose Down
DROP TABLE IF EXISTS devices;
