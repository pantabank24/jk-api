-- Manual price overrides with a validity window. A manual row (source=manual)
-- with valid_from <= now <= valid_until takes precedence over auto-fetched
-- prices; once the window passes the system falls back to the latest auto price.

-- gold_prices: needs a source marker + the window.
ALTER TABLE gold_prices ADD COLUMN IF NOT EXISTS source      VARCHAR(20) DEFAULT 'auto';
ALTER TABLE gold_prices ADD COLUMN IF NOT EXISTS valid_from  TIMESTAMP WITH TIME ZONE;
ALTER TABLE gold_prices ADD COLUMN IF NOT EXISTS valid_until TIMESTAMP WITH TIME ZONE;

-- metal_prices already has source; just add the window.
ALTER TABLE metal_prices ADD COLUMN IF NOT EXISTS valid_from  TIMESTAMP WITH TIME ZONE;
ALTER TABLE metal_prices ADD COLUMN IF NOT EXISTS valid_until TIMESTAMP WITH TIME ZONE;

CREATE INDEX IF NOT EXISTS idx_gold_prices_manual  ON gold_prices(source, valid_until);
CREATE INDEX IF NOT EXISTS idx_metal_prices_manual ON metal_prices(symbol, source, valid_until);
