-- no_header marks a quotation intentionally issued WITHOUT a receipt header
-- (master/owner opt-out). Distinguishes it from legacy quotations whose header
-- snapshot is empty because it predates the snapshot columns — readers fall
-- back to the live store relation for those, but must not for no_header docs.
ALTER TABLE quotations ADD COLUMN IF NOT EXISTS no_header BOOLEAN NOT NULL DEFAULT false;
