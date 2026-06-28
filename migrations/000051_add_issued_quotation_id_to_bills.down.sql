DROP INDEX IF EXISTS idx_quotations_issued_quotation_id;
ALTER TABLE quotations DROP COLUMN IF EXISTS issued_quotation_id;
