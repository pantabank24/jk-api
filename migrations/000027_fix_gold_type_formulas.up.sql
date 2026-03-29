-- Fix formula_steps for all gold types.
-- Uses "number" operand instead of "service" so service_rate is not needed.
-- Safe UPDATE: only touches formula_steps and service_rate, preserves all other data.

-- 1. ทองคำแท่ง 96.5%: price / 15.2
UPDATE gold_types SET
  formula_steps = '[{"operator":"/","operand_type":"number","value":15.2}]',
  service_rate  = 0
WHERE name = 'ทองคำแท่ง 96.5%';

-- 2. ทองรูปพรรณ: price * 0.0656 * 0.965
UPDATE gold_types SET
  formula_steps = '[{"operator":"*","operand_type":"number","value":0.0656},{"operator":"*","operand_type":"number","value":0.965}]',
  service_rate  = 0.0656
WHERE name = 'ทองรูปพรรณ';

-- 3. ทองหลอม: (price + plus) * 0.0656 * percent / 100
UPDATE gold_types SET
  formula_steps = '[{"operator":"+","operand_type":"plus","value":0},{"operator":"*","operand_type":"number","value":0.0656},{"operator":"*","operand_type":"percent","value":0},{"operator":"/","operand_type":"number","value":100}]',
  service_rate  = 0.0656,
  default_percent = 90
WHERE name = 'ทองหลอม';

-- 4. กรอบทอง/ตลับทอง: price * 0.0656 * percent / 100
UPDATE gold_types SET
  formula_steps = '[{"operator":"*","operand_type":"number","value":0.0656},{"operator":"*","operand_type":"percent","value":0},{"operator":"/","operand_type":"number","value":100}]',
  service_rate  = 0.0656,
  default_percent = 90
WHERE name = 'กรอบทอง/ตลับทอง';

-- 5. ทอง 9K: price * 0.0656 * 0.375
UPDATE gold_types SET
  formula_steps = '[{"operator":"*","operand_type":"number","value":0.0656},{"operator":"*","operand_type":"number","value":0.375}]',
  service_rate  = 0.0656
WHERE name = 'ทอง 9K';

-- 6. ทอง 14K: price * 0.0656 * 0.585
UPDATE gold_types SET
  formula_steps = '[{"operator":"*","operand_type":"number","value":0.0656},{"operator":"*","operand_type":"number","value":0.585}]',
  service_rate  = 0.0656
WHERE name = 'ทอง 14K';

-- 7. ทอง 18K: price * 0.0656 * 0.75
UPDATE gold_types SET
  formula_steps = '[{"operator":"*","operand_type":"number","value":0.0656},{"operator":"*","operand_type":"number","value":0.75}]',
  service_rate  = 0.0656
WHERE name = 'ทอง 18K';

-- 8. อื่น ๆ: price * 0.0656 * percent / 100
UPDATE gold_types SET
  formula_steps = '[{"operator":"*","operand_type":"number","value":0.0656},{"operator":"*","operand_type":"percent","value":0},{"operator":"/","operand_type":"number","value":100}]',
  service_rate  = 0.0656,
  default_percent = 90
WHERE name = 'อื่น ๆ';
