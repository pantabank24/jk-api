-- Fold the two meters back into the single one migration 89 created. Gold wins:
-- it is the metal that was configured before the split, and silver has no place
-- to go in a single-metal schema.

INSERT INTO system_configs (key, value, description)
SELECT 'line_sell_accum_enabled',
       COALESCE((SELECT value FROM system_configs WHERE key = 'line_sell_accum_gold_enabled'), 'false'),
       'LINE: แจ้งเตือนเมื่อยอดทองที่ลูกค้าขายเข้ามาสะสมครบเกณฑ์ (true/false)'
ON CONFLICT (key) DO NOTHING;

INSERT INTO system_configs (key, value, description)
SELECT 'line_sell_accum_threshold',
       COALESCE(NULLIF((SELECT value FROM system_configs WHERE key = 'line_sell_accum_threshold_gold'), ''), '65'),
       'LINE: น้ำหนักสะสม (บาท) ต่อ 1 กิโลกรัมที่จะแจ้งขาย (0 = ปิด)'
ON CONFLICT (key) DO NOTHING;

INSERT INTO system_configs (key, value, description)
SELECT 'line_sell_accum_purity',
       COALESCE(NULLIF((SELECT value FROM system_configs WHERE key = 'line_sell_accum_purity_gold'), ''), '99.99'),
       'LINE: ความบริสุทธิ์ (%) ที่ระบุในข้อความแจ้งขาย'
ON CONFLICT (key) DO NOTHING;

INSERT INTO system_configs (key, value, description)
SELECT 'line_sell_accum_weight',
       COALESCE(NULLIF((SELECT value FROM system_configs WHERE key = 'line_sell_accum_weight_gold'), ''), '0'),
       'LINE: ภายใน — น้ำหนักสะสม (บาท) ที่ยังไม่ครบเกณฑ์'
ON CONFLICT (key) DO NOTHING;

DELETE FROM system_configs WHERE key IN (
  'line_sell_accum_gold_enabled',
  'line_sell_accum_silver_enabled',
  'line_sell_accum_threshold_gold',
  'line_sell_accum_threshold_silver',
  'line_sell_accum_purity_gold',
  'line_sell_accum_purity_silver',
  'line_sell_accum_weight_gold',
  'line_sell_accum_weight_silver'
);
