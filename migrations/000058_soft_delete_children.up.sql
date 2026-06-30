-- Soft-delete support for child records so deleting a quotation/bill/store can
-- cascade-soft-delete its children, and deleted bills drop out of debt totals.

ALTER TABLE quotation_items     ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMPTZ;
ALTER TABLE quotation_images    ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMPTZ;
ALTER TABLE bill_balances       ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMPTZ;
ALTER TABLE bill_delivery_logs  ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMPTZ;

CREATE INDEX IF NOT EXISTS idx_quotation_items_deleted_at    ON quotation_items(deleted_at);
CREATE INDEX IF NOT EXISTS idx_quotation_images_deleted_at   ON quotation_images(deleted_at);
CREATE INDEX IF NOT EXISTS idx_bill_balances_deleted_at      ON bill_balances(deleted_at);
CREATE INDEX IF NOT EXISTS idx_bill_delivery_logs_deleted_at ON bill_delivery_logs(deleted_at);
