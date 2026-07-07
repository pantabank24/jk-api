-- Tag each quotation/bill item with its metal (gold|silver|platinum|palladium).
-- Existing rows are gold — the system only handled gold before this column.
ALTER TABLE quotation_items ADD COLUMN IF NOT EXISTS metal VARCHAR(20) NOT NULL DEFAULT 'gold';
