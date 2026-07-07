-- Receipt header moves from stores to branches: each branch prints its own
-- header (name, address, phone, tax info, website subtitle, logo). address and
-- phone already exist on branches; add the rest plus a per-store main flag.
ALTER TABLE branches ADD COLUMN IF NOT EXISTS header_name VARCHAR(255) NOT NULL DEFAULT '';
ALTER TABLE branches ADD COLUMN IF NOT EXISTS tax_id      VARCHAR(50)  NOT NULL DEFAULT '';
ALTER TABLE branches ADD COLUMN IF NOT EXISTS tax_name    VARCHAR(255) NOT NULL DEFAULT '';
ALTER TABLE branches ADD COLUMN IF NOT EXISTS website     VARCHAR(255) NOT NULL DEFAULT '';
ALTER TABLE branches ADD COLUMN IF NOT EXISTS logo        VARCHAR(500) NOT NULL DEFAULT '';
ALTER TABLE branches ADD COLUMN IF NOT EXISTS is_main     BOOLEAN      NOT NULL DEFAULT false;

-- Backfill: copy each store's existing header onto its main branch (the
-- lowest-id branch of the store) and flag it as the store's main branch. The
-- store's own name becomes the branch's header (shop) name; the branch keeps
-- its own name for the "สาขา" line. Only fill address/phone from the store when
-- the branch hasn't set its own.
WITH main_branch AS (
    SELECT DISTINCT ON (store_id) id, store_id
    FROM branches
    WHERE deleted_at IS NULL
    ORDER BY store_id, id ASC
)
UPDATE branches b
SET header_name = s.name,
    address     = CASE WHEN b.address = '' THEN s.address ELSE b.address END,
    phone       = CASE WHEN b.phone   = '' THEN s.phone   ELSE b.phone   END,
    tax_id      = s.tax_id,
    tax_name    = s.tax_name,
    website     = s.website,
    logo        = s.logo,
    is_main     = true
FROM stores s, main_branch mb
WHERE b.id = mb.id AND s.id = b.store_id;

-- Stores that have no branch yet: create a main branch from the store's info so
-- the header isn't lost. Uses a store-id-based code to avoid colliding with the
-- BRN#### codes the app generates.
INSERT INTO branches (store_id, code, name, header_name, address, phone, tax_id, tax_name, website, logo, is_main, is_active, created_at, updated_at)
SELECT s.id,
       'BRNH' || LPAD(s.id::text, 4, '0'),
       'สำนักงานใหญ่',
       s.name, s.address, s.phone, s.tax_id, s.tax_name, s.website, s.logo,
       true, true, NOW(), NOW()
FROM stores s
WHERE s.deleted_at IS NULL
  AND NOT EXISTS (
      SELECT 1 FROM branches b WHERE b.store_id = s.id AND b.deleted_at IS NULL
  );
