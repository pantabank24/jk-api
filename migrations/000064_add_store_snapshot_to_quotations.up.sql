-- Snapshot of the store/branch header at the time the quotation was created,
-- so reprinting an old quotation later (after the store's info changes) still
-- shows the header as it was on the day the quotation was issued.
ALTER TABLE quotations ADD COLUMN IF NOT EXISTS store_name VARCHAR(255) NOT NULL DEFAULT '';
ALTER TABLE quotations ADD COLUMN IF NOT EXISTS store_branch VARCHAR(255) NOT NULL DEFAULT '';
ALTER TABLE quotations ADD COLUMN IF NOT EXISTS store_address TEXT NOT NULL DEFAULT '';
ALTER TABLE quotations ADD COLUMN IF NOT EXISTS store_phone VARCHAR(20) NOT NULL DEFAULT '';
ALTER TABLE quotations ADD COLUMN IF NOT EXISTS store_tax_id VARCHAR(50) NOT NULL DEFAULT '';
ALTER TABLE quotations ADD COLUMN IF NOT EXISTS store_tax_name VARCHAR(255) NOT NULL DEFAULT '';
ALTER TABLE quotations ADD COLUMN IF NOT EXISTS store_website VARCHAR(255) NOT NULL DEFAULT '';
ALTER TABLE quotations ADD COLUMN IF NOT EXISTS store_logo VARCHAR(500) NOT NULL DEFAULT '';
