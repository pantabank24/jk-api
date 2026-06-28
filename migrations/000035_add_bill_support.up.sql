-- Bills reuse the quotations table. is_bill distinguishes customer-created
-- bills (status 10-13) from staff quotations (status 0-2).
ALTER TABLE quotations ADD COLUMN IF NOT EXISTS is_bill BOOLEAN NOT NULL DEFAULT FALSE;

CREATE INDEX IF NOT EXISTS idx_quotations_is_bill ON quotations(is_bill);
