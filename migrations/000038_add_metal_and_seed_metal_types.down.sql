DELETE FROM gold_types WHERE name IN ('เงินแท่ง', 'แพลตินัม', 'แพลเลเดียม');
ALTER TABLE gold_types DROP COLUMN IF EXISTS metal;
