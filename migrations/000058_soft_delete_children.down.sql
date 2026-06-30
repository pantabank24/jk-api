DROP INDEX IF EXISTS idx_quotation_items_deleted_at;
DROP INDEX IF EXISTS idx_quotation_images_deleted_at;
DROP INDEX IF EXISTS idx_bill_balances_deleted_at;
DROP INDEX IF EXISTS idx_bill_delivery_logs_deleted_at;

ALTER TABLE quotation_items     DROP COLUMN IF EXISTS deleted_at;
ALTER TABLE quotation_images    DROP COLUMN IF EXISTS deleted_at;
ALTER TABLE bill_balances       DROP COLUMN IF EXISTS deleted_at;
ALTER TABLE bill_delivery_logs  DROP COLUMN IF EXISTS deleted_at;
