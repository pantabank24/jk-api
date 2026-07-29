DROP INDEX IF EXISTS idx_quotations_bill_metal;
ALTER TABLE quotations DROP COLUMN IF EXISTS metal;
