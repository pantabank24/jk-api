-- Record which gold-price round a quotation/bill was created in (for reporting).
-- gold_round mirrors gold_prices.gold_round (e.g. "(ครั้งที่ 1)"); gold_price_id
-- links the exact price snapshot used.
ALTER TABLE quotations ADD COLUMN IF NOT EXISTS gold_round VARCHAR(50) NOT NULL DEFAULT '';
ALTER TABLE quotations ADD COLUMN IF NOT EXISTS gold_price_id BIGINT REFERENCES gold_prices(id) ON DELETE SET NULL;

CREATE INDEX IF NOT EXISTS idx_quotations_gold_price_id ON quotations(gold_price_id);
