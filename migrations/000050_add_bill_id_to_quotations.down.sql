DROP INDEX IF EXISTS idx_quotations_bill_id;
ALTER TABLE quotations DROP COLUMN IF EXISTS bill_id;
