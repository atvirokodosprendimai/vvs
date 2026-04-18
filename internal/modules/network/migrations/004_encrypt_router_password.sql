-- +goose Up
-- +goose StatementBegin
ALTER TABLE routers ADD COLUMN password_enc BLOB NULL;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
SELECT 1; -- no-op; cannot drop columns in SQLite
-- +goose StatementEnd
