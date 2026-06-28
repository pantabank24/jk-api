-- Generalise gold_types into "product types" of any metal. Existing rows are
-- gold (covered by the default).
ALTER TABLE gold_types ADD COLUMN IF NOT EXISTS metal VARCHAR(20) NOT NULL DEFAULT 'gold';

-- Seed silver / platinum / palladium product types. Formulas mirror jk-goldtrader:
--   silver:    (price / 1000) * (percent / 100) * weight
--   platinum:  price * (percent / 100) * weight   (price entered at quotation time)
--   palladium: price * (percent / 100) * weight   (price entered at quotation time)
-- price_source 'manual' means the create screen does NOT auto-fill the price.
DELETE FROM gold_types WHERE name IN ('เงินแท่ง', 'แพลตินัม', 'แพลเลเดียม');

INSERT INTO gold_types (name, description, metal, price_source, default_percent, default_plus, formula_steps, service_rate, plus_type, sort_order) VALUES
('เงินแท่ง', 'เงินแท่ง (ราคา ÷ 1000 × %)', 'silver', 'buy', 99.9, 0,
 '[{"operator":"/","operand_type":"number","value":1000},{"operator":"*","operand_type":"percent","value":0},{"operator":"/","operand_type":"number","value":100}]',
 0, 0, 10),

('แพลตินัม', 'แพลตินัม (กรอกราคาเอง × %)', 'platinum', 'manual', 95, 0,
 '[{"operator":"*","operand_type":"percent","value":0},{"operator":"/","operand_type":"number","value":100}]',
 0, 0, 11),

('แพลเลเดียม', 'แพลเลเดียม (กรอกราคาเอง × %)', 'palladium', 'manual', 95, 0,
 '[{"operator":"*","operand_type":"percent","value":0},{"operator":"/","operand_type":"number","value":100}]',
 0, 0, 12);
