-- +goose Up
-- SQLite does not support ALTER TABLE ... MODIFY COLUMN.
-- Recreate the users table with the updated CHECK constraint to allow 'viewer' role.
CREATE TABLE users_new (
    id            TEXT PRIMARY KEY,
    username      TEXT NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,
    role          TEXT NOT NULL CHECK (role IN ('admin', 'operator', 'viewer')),
    created_at    DATETIME NOT NULL,
    updated_at    DATETIME NOT NULL
);

INSERT INTO users_new SELECT id, username, password_hash, role, created_at, updated_at FROM users;

DROP TABLE users;

ALTER TABLE users_new RENAME TO users;

-- +goose Down
CREATE TABLE users_old (
    id            TEXT PRIMARY KEY,
    username      TEXT NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,
    role          TEXT NOT NULL CHECK (role IN ('admin', 'operator')),
    created_at    DATETIME NOT NULL,
    updated_at    DATETIME NOT NULL
);

-- viewer accounts are dropped on rollback
INSERT INTO users_old
    SELECT id, username, password_hash, role, created_at, updated_at
    FROM users WHERE role IN ('admin', 'operator');

DROP TABLE users;

ALTER TABLE users_old RENAME TO users;
