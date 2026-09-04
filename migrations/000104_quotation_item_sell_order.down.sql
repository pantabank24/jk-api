DROP INDEX IF EXISTS idx_quotation_items_sell_order;
ALTER TABLE quotation_items DROP COLUMN IF EXISTS sell_order_id;
