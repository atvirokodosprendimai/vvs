-- +goose Up
CREATE TABLE IF NOT EXISTS vm_plans (
    id                       TEXT PRIMARY KEY,
    name                     TEXT NOT NULL,
    description              TEXT NOT NULL DEFAULT '',
    cores                    INTEGER NOT NULL,
    memory_mb                INTEGER NOT NULL,
    disk_gb                  INTEGER NOT NULL,
    storage                  TEXT NOT NULL DEFAULT 'local-lvm',
    template_vmid            INTEGER NOT NULL,
    node_id                  TEXT NOT NULL DEFAULT '',
    price_monthly_euro_cents INTEGER NOT NULL DEFAULT 0,
    enabled                  BOOLEAN NOT NULL DEFAULT 1,
    notes                    TEXT NOT NULL DEFAULT '',
    created_at               DATETIME NOT NULL,
    updated_at               DATETIME NOT NULL
);

-- +goose Down
DROP TABLE IF EXISTS vm_plans;
