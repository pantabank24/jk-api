-- A customer bill points to the master-issued quotation it was rolled into.
-- Many bills can share one issued quotation (master combines all pending bills).
ALTER TABLE quotations ADD COLUMN IF NOT EXISTS issued_quotation_id BIGINT REFERENCES quotations(id) ON DELETE SET NULL;
CREATE INDEX IF NOT EXISTS idx_quotations_issued_quotation_id ON quotations(issued_quotation_id);
