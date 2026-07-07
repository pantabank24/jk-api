-- Drop the branch-level receipt-header columns. The store columns still hold the
-- original values, so reverting restores the store-based header. Branches that
-- were auto-created for storeless stores are left in place (harmless).
ALTER TABLE branches DROP COLUMN IF EXISTS header_name;
ALTER TABLE branches DROP COLUMN IF EXISTS tax_id;
ALTER TABLE branches DROP COLUMN IF EXISTS tax_name;
ALTER TABLE branches DROP COLUMN IF EXISTS website;
ALTER TABLE branches DROP COLUMN IF EXISTS logo;
ALTER TABLE branches DROP COLUMN IF EXISTS is_main;
