-- Customer tax ID (เลขประจำตัวผู้เสียภาษี), shown on issued quotations.
ALTER TABLE users ADD COLUMN IF NOT EXISTS tax_id VARCHAR(50) NOT NULL DEFAULT '';
