DROP INDEX IF EXISTS idx_quotations_after_melt_cleared_at;

ALTER TABLE quotations
    DROP COLUMN IF EXISTS after_melt_cleared_at;
