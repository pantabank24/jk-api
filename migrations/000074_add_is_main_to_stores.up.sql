-- Main store flag (one per system) — mirrors branches.is_main. Used as the
-- default store for master users, e.g. the receipt-header picker on the
-- quotation page, which previously started empty for masters.
ALTER TABLE stores ADD COLUMN IF NOT EXISTS is_main BOOLEAN NOT NULL DEFAULT false;

-- Backfill: flag the oldest active store as main so masters get a default
-- immediately (can be changed later in the store edit page).
UPDATE stores
SET is_main = true
WHERE id = (
    SELECT id FROM stores
    WHERE deleted_at IS NULL
    ORDER BY id ASC
    LIMIT 1
);
