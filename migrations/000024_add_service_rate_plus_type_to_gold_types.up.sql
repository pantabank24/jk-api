-- Add service_rate and plus_type columns
ALTER TABLE gold_types ADD COLUMN IF NOT EXISTS service_rate DECIMAL(15,8) NOT NULL DEFAULT 0;
ALTER TABLE gold_types ADD COLUMN IF NOT EXISTS plus_type    INT          NOT NULL DEFAULT 0; -- 0=บาท, 1=%

-- Remove old seed types from migration 000016 (safe if no quotation_items reference them)
DELETE FROM gold_types WHERE name IN (
    'ทองคำแท่ง', 'ทองรูปพรรณ', 'ทองเก่า', 'เงิน', 'แพลตินัม', 'แพลเลเดียม'
);

-- Seed 8 gold types based on jk-goldtrader home.tsx formulas
-- Formula starts at price and applies steps sequentially.
-- operand_type "service" uses gt.service_rate at runtime.

INSERT INTO gold_types (name, description, price_source, default_percent, default_plus, formula_steps, service_rate, plus_type, sort_order) VALUES

-- 1. ทองคำแท่ง 96.5%: perGram = price / 15.2
('ทองคำแท่ง 96.5%', 'ทองคำแท่ง 96.5% (บาทละ → ต่อกรัม)', 'bar_buy', 0, 0,
 '[{"operator":"/","operand_type":"number","value":15.2}]',
 1.0, 0, 1),

-- 2. ทองรูปพรรณ: perGram = price * service * 0.965
('ทองรูปพรรณ', 'ทองรูปพรรณ 96.5%', 'ornament_buy', 0, 0,
 '[{"operator":"*","operand_type":"service","value":0},{"operator":"*","operand_type":"number","value":0.965}]',
 0.0658, 0, 2),

-- 3. ทองหลอม: perGram = (price + plus) * service * (percent / 100)
('ทองหลอม', 'ทองหลอม / ทองบริสุทธิ์ (plus คือราคาบวก, percent คือความบริสุทธิ์)', 'bar_buy', 90, 0,
 '[{"operator":"+","operand_type":"plus","value":0},{"operator":"*","operand_type":"service","value":0},{"operator":"*","operand_type":"percent","value":0},{"operator":"/","operand_type":"number","value":100}]',
 0.0658, 0, 3),

-- 4. กรอบทอง/ตลับทอง: perGram = price * service * (percent / 100)
('กรอบทอง/ตลับทอง', 'กรอบทองและตลับทอง', 'bar_buy', 90, 0,
 '[{"operator":"*","operand_type":"service","value":0},{"operator":"*","operand_type":"percent","value":0},{"operator":"/","operand_type":"number","value":100}]',
 0.0658, 0, 4),

-- 5. ทอง 9K: perGram = price * service * 0.375
('ทอง 9K', 'ทอง 9 กะรัต (37.5%)', 'bar_buy', 0, 0,
 '[{"operator":"*","operand_type":"service","value":0},{"operator":"*","operand_type":"number","value":0.375}]',
 0.0658, 0, 5),

-- 6. ทอง 14K: perGram = price * service * 0.585
('ทอง 14K', 'ทอง 14 กะรัต (58.5%)', 'bar_buy', 0, 0,
 '[{"operator":"*","operand_type":"service","value":0},{"operator":"*","operand_type":"number","value":0.585}]',
 0.0658, 0, 6),

-- 7. ทอง 18K: perGram = price * service * 0.75
('ทอง 18K', 'ทอง 18 กะรัต (75%)', 'bar_buy', 0, 0,
 '[{"operator":"*","operand_type":"service","value":0},{"operator":"*","operand_type":"number","value":0.75}]',
 0.0658, 0, 7),

-- 8. อื่น ๆ: perGram = price * service * (percent / 100)
('อื่น ๆ', 'ประเภทอื่น ๆ กำหนดเอง', 'bar_buy', 90, 0,
 '[{"operator":"*","operand_type":"service","value":0},{"operator":"*","operand_type":"percent","value":0},{"operator":"/","operand_type":"number","value":100}]',
 0.0658, 0, 8);
