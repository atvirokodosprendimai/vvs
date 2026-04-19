-- +goose Up

-- Roles registry: all valid roles in the system.
-- is_builtin=1 rows cannot be deleted via the admin UI.
-- can_write=1 means users with this role may perform mutations (RequireWrite passes).
CREATE TABLE roles (
    name         TEXT    NOT NULL PRIMARY KEY,
    display_name TEXT    NOT NULL DEFAULT '',
    is_builtin   INTEGER NOT NULL DEFAULT 0,
    can_write    INTEGER NOT NULL DEFAULT 1
);

-- Seed built-in roles.
INSERT INTO roles (name, display_name, is_builtin, can_write) VALUES
    ('admin',    'Administrator', 1, 1),
    ('operator', 'Operator',      1, 1),
    ('viewer',   'Viewer',        1, 0);

-- Seed missing iptv module permissions for existing roles.
INSERT OR IGNORE INTO role_module_permissions (role, module, can_view, can_edit) VALUES
    ('operator', 'iptv', 1, 1),
    ('viewer',   'iptv', 1, 0);

-- Drop the hardcoded CHECK constraint on users.role so custom roles can be assigned.
-- SQLite requires table recreation to remove a constraint.
CREATE TABLE users_new (
    id            TEXT PRIMARY KEY,
    username      TEXT NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,
    role          TEXT NOT NULL DEFAULT 'viewer',
    full_name     TEXT NOT NULL DEFAULT '',
    division      TEXT NOT NULL DEFAULT '',
    totp_secret   TEXT NOT NULL DEFAULT '',
    totp_enabled  INTEGER NOT NULL DEFAULT 0,
    created_at    DATETIME NOT NULL,
    updated_at    DATETIME NOT NULL
);

INSERT INTO users_new SELECT id, username, password_hash, role, full_name, division,
    totp_secret, totp_enabled, created_at, updated_at FROM users;

DROP TABLE users;

ALTER TABLE users_new RENAME TO users;

-- +goose Down
DELETE FROM role_module_permissions WHERE module = 'iptv';
DROP TABLE IF EXISTS roles;
-- Restore CHECK constraint (down migration does NOT recreate the constraint
-- since rolling back to a 3-role world with custom users would be destructive).
-- Operators should not roll back this migration in production.
