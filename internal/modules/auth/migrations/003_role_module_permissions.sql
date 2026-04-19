-- +goose Up
CREATE TABLE role_module_permissions (
    role       TEXT NOT NULL,
    module     TEXT NOT NULL,
    can_view   INTEGER NOT NULL DEFAULT 1,
    can_edit   INTEGER NOT NULL DEFAULT 1,
    PRIMARY KEY (role, module)
);

-- Seed defaults for operator: full view + edit on all modules
INSERT INTO role_module_permissions (role, module, can_view, can_edit) VALUES
    ('operator', 'customers', 1, 1),
    ('operator', 'tickets',   1, 1),
    ('operator', 'deals',     1, 1),
    ('operator', 'tasks',     1, 1),
    ('operator', 'contacts',  1, 1),
    ('operator', 'invoices',  1, 1),
    ('operator', 'products',  1, 1),
    ('operator', 'payments',  1, 1),
    ('operator', 'network',   1, 1),
    ('operator', 'email',     1, 1),
    ('operator', 'cron',      1, 1),
    ('operator', 'audit_log', 1, 1),
    ('operator', 'users',     0, 0);

-- Seed defaults for viewer: view-only on all modules, no users access
INSERT INTO role_module_permissions (role, module, can_view, can_edit) VALUES
    ('viewer', 'customers', 1, 0),
    ('viewer', 'tickets',   1, 0),
    ('viewer', 'deals',     1, 0),
    ('viewer', 'tasks',     1, 0),
    ('viewer', 'contacts',  1, 0),
    ('viewer', 'invoices',  1, 0),
    ('viewer', 'products',  1, 0),
    ('viewer', 'payments',  1, 0),
    ('viewer', 'network',   1, 0),
    ('viewer', 'email',     1, 0),
    ('viewer', 'cron',      1, 0),
    ('viewer', 'audit_log', 1, 0),
    ('viewer', 'users',     0, 0);

-- +goose Down
DROP TABLE IF EXISTS role_module_permissions;
