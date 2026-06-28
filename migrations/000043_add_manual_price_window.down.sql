DROP INDEX IF EXISTS idx_metal_prices_manual;
DROP INDEX IF EXISTS idx_gold_prices_manual;
ALTER TABLE metal_prices DROP COLUMN IF EXISTS valid_until;
ALTER TABLE metal_prices DROP COLUMN IF EXISTS valid_from;
ALTER TABLE gold_prices DROP COLUMN IF EXISTS valid_until;
ALTER TABLE gold_prices DROP COLUMN IF EXISTS valid_from;
ALTER TABLE gold_prices DROP COLUMN IF EXISTS source;
