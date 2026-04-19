-- +goose Up
CREATE INDEX IF NOT EXISTS idx_contacts_email ON contacts(email);

-- +goose Down
DROP INDEX IF EXISTS idx_contacts_email;
