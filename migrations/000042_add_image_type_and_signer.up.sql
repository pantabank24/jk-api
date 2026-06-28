-- Categorise quotation images by type (before_melt / after_melt / signature)
-- and record the signer's name on the quotation.
ALTER TABLE quotation_images ADD COLUMN IF NOT EXISTS type VARCHAR(50) NOT NULL DEFAULT '';
ALTER TABLE quotations ADD COLUMN IF NOT EXISTS signer_name VARCHAR(255) NOT NULL DEFAULT '';
