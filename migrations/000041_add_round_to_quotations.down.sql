DROP INDEX IF EXISTS idx_quotations_gold_price_id;
ALTER TABLE quotations DROP COLUMN IF EXISTS gold_price_id;
ALTER TABLE quotations DROP COLUMN IF EXISTS gold_round;
