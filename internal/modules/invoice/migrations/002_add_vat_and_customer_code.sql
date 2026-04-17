-- +goose Up
ALTER TABLE invoices ADD COLUMN customer_code TEXT NOT NULL DEFAULT '';
ALTER TABLE invoices ADD COLUMN sub_total INTEGER NOT NULL DEFAULT 0;
ALTER TABLE invoices ADD COLUMN vat_total INTEGER NOT NULL DEFAULT 0;

ALTER TABLE invoice_line_items ADD COLUMN unit_price_gross INTEGER NOT NULL DEFAULT 0;
ALTER TABLE invoice_line_items ADD COLUMN vat_rate INTEGER NOT NULL DEFAULT 21;
ALTER TABLE invoice_line_items ADD COLUMN total_vat INTEGER NOT NULL DEFAULT 0;
ALTER TABLE invoice_line_items ADD COLUMN total_gross INTEGER NOT NULL DEFAULT 0;

-- Backfill existing rows: treat existing unit_price as gross with 0% VAT
UPDATE invoice_line_items SET unit_price_gross = unit_price, vat_rate = 0, total_gross = total_price, total_vat = 0;
UPDATE invoices SET sub_total = total_amount, vat_total = 0;

-- +goose Down
-- SQLite doesn't support DROP COLUMN before 3.35.0; these are safe on modern SQLite.
ALTER TABLE invoices DROP COLUMN customer_code;
ALTER TABLE invoices DROP COLUMN sub_total;
ALTER TABLE invoices DROP COLUMN vat_total;

ALTER TABLE invoice_line_items DROP COLUMN unit_price_gross;
ALTER TABLE invoice_line_items DROP COLUMN vat_rate;
ALTER TABLE invoice_line_items DROP COLUMN total_vat;
ALTER TABLE invoice_line_items DROP COLUMN total_gross;
