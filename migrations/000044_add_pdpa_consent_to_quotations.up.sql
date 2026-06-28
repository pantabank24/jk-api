-- Record the PDPA consent given when a quotation is created (required at creation).
ALTER TABLE quotations ADD COLUMN IF NOT EXISTS pdpa_consent BOOLEAN NOT NULL DEFAULT FALSE;
