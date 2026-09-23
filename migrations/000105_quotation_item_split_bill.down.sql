DROP INDEX IF EXISTS idx_quotation_items_split_bill;
ALTER TABLE quotation_items DROP COLUMN IF EXISTS split_bill_id;
