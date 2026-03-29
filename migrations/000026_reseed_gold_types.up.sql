-- Re-ensure gold type formula_steps and service_rate are correct.
-- Forces correct values for the 8 seeded types.

-- Ensure columns exist (safe no-op if already added)
ALTER TABLE gold_types ADD COLUMN IF NOT EXISTS formula_steps TEXT NOT NULL DEFAULT '[]';
ALTER TABLE gold_types ADD COLUMN IF NOT EXISTS service_rate DECIMAL(15,8) NOT NULL DEFAULT 0;
ALTER TABLE gold_types ADD COLUMN IF NOT EXISTS plus_type INT NOT NULL DEFAULT 0;

-- Remove any lingering old types from migration 000016
DELETE FROM gold_types WHERE name IN ('ทองคำแท่ง', 'ทองเก่า', 'เงิน', 'แพลตินัม', 'แพลเลเดียม');

-- Remove and re-insert all 8 current types to guarantee correct data
DELETE FROM gold_types WHERE name IN (
    'ทองคำแท่ง 96.5%', 'ทองรูปพรรณ', 'ทองหลอม', 'กรอบทอง/ตลับทอง',
    'ทอง 9K', 'ทอง 14K', 'ทอง 18K', 'อื่น ๆ'
);

INSERT INTO gold_types (name, description, price_source, default_percent, default_plus, formula_steps, service_rate, plus_type, sort_order) VALUES
('ทองคำแท่ง 96.5%', 'ทองคำแท่ง 96.5% (บาทละ → ต่อกรัม)', 'bar_buy', 0, 0,
 '[{"operator":"/","operand_type":"number","value":15.2}]',
 1.0, 0, 1),

('ทองรูปพรรณ', 'ทองรูปพรรณ 96.5%', 'ornament_buy', 0, 0,
 '[{"operator":"*","operand_type":"service","value":0},{"operator":"*","operand_type":"number","value":0.965}]',
 0.0656, 0, 2),

('ทองหลอม', 'ทองหลอม / ทองบริสุทธิ์ (plus คือราคาบวก, percent คือความบริสุทธิ์)', 'bar_buy', 90, 0,
 '[{"operator":"+","operand_type":"plus","value":0},{"operator":"*","operand_type":"service","value":0},{"operator":"*","operand_type":"percent","value":0},{"operator":"/","operand_type":"number","value":100}]',
 0.0656, 0, 3),

('กรอบทอง/ตลับทอง', 'กรอบทองและตลับทอง', 'bar_buy', 90, 0,
 '[{"operator":"*","operand_type":"service","value":0},{"operator":"*","operand_type":"percent","value":0},{"operator":"/","operand_type":"number","value":100}]',
 0.0656, 0, 4),

('ทอง 9K', 'ทอง 9 กะรัต (37.5%)', 'bar_buy', 0, 0,
 '[{"operator":"*","operand_type":"service","value":0},{"operator":"*","operand_type":"number","value":0.375}]',
 0.0656, 0, 5),

('ทอง 14K', 'ทอง 14 กะรัต (58.5%)', 'bar_buy', 0, 0,
 '[{"operator":"*","operand_type":"service","value":0},{"operator":"*","operand_type":"number","value":0.585}]',
 0.0656, 0, 6),

('ทอง 18K', 'ทอง 18 กะรัต (75%)', 'bar_buy', 0, 0,
 '[{"operator":"*","operand_type":"service","value":0},{"operator":"*","operand_type":"number","value":0.75}]',
 0.0656, 0, 7),

('อื่น ๆ', 'ประเภทอื่น ๆ กำหนดเอง', 'bar_buy', 90, 0,
 '[{"operator":"*","operand_type":"service","value":0},{"operator":"*","operand_type":"percent","value":0},{"operator":"/","operand_type":"number","value":100}]',
 0.0656, 0, 8);
