-- Taxpayer name shown in the receipt-header tax block (separate from the shop's
-- display name).
ALTER TABLE stores ADD COLUMN IF NOT EXISTS tax_name VARCHAR(255) DEFAULT '';
