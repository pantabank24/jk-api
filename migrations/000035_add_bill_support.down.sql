DROP INDEX IF EXISTS idx_quotations_is_bill;
ALTER TABLE quotations DROP COLUMN IF EXISTS is_bill;
