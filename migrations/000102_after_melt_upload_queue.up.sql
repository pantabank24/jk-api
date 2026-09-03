ALTER TABLE quotations
    ADD COLUMN IF NOT EXISTS after_melt_cleared_at TIMESTAMPTZ;

CREATE INDEX IF NOT EXISTS idx_quotations_after_melt_cleared_at
    ON quotations(after_melt_cleared_at);
