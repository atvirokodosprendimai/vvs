-- +goose Up
ALTER TABLE users ADD COLUMN totp_secret  TEXT    NOT NULL DEFAULT '';
ALTER TABLE users ADD COLUMN totp_enabled INTEGER NOT NULL DEFAULT 0;

-- +goose Down
-- SQLite does not support DROP COLUMN in older versions; use a table rebuild if needed.
-- For rollback: no-op (columns remain but are unused).
