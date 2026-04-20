-- +goose Up
CREATE TABLE proxmox_vms (
    id          TEXT PRIMARY KEY,
    vmid        INTEGER NOT NULL,
    node_id     TEXT NOT NULL REFERENCES proxmox_nodes(id),
    customer_id TEXT,
    name        TEXT NOT NULL,
    status      TEXT NOT NULL DEFAULT 'unknown',
    cores       INTEGER NOT NULL DEFAULT 1,
    memory_mb   INTEGER NOT NULL DEFAULT 1024,
    disk_gb     INTEGER NOT NULL DEFAULT 10,
    ip_address  TEXT NOT NULL DEFAULT '',
    notes       TEXT NOT NULL DEFAULT '',
    created_at  DATETIME NOT NULL,
    updated_at  DATETIME NOT NULL
);

CREATE UNIQUE INDEX idx_proxmox_vms_vmid_node ON proxmox_vms(vmid, node_id);
CREATE INDEX idx_proxmox_vms_customer ON proxmox_vms(customer_id);

-- +goose Down
DROP INDEX IF EXISTS idx_proxmox_vms_customer;
DROP INDEX IF EXISTS idx_proxmox_vms_vmid_node;
DROP TABLE proxmox_vms;
