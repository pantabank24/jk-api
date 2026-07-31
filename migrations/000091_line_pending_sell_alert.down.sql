-- Back to the shop-wide meter of migrations 89/90. The meters restart at 0:
-- the per-customer alert never tracked a running total, so there is nothing to
-- carry back into them.

INSERT INTO system_configs (key, value, description)
SELECT 'line_sell_accum_gold_enabled',
       COALESCE((SELECT value FROM system_configs WHERE key = 'line_pending_sell_gold_enabled'), 'false'),
       'LINE: แจ้งเตือนเมื่อยอดทองที่ลูกค้าขายเข้ามาสะสมครบเกณฑ์ (true/false)'
ON CONFLICT (key) DO NOTHING;

INSERT INTO system_configs (key, value, description)
SELECT 'line_sell_accum_silver_enabled',
       COALESCE((SELECT value FROM system_configs WHERE key = 'line_pending_sell_silver_enabled'), 'false'),
       'LINE: แจ้งเตือนเมื่อยอดเงินที่ลูกค้าขายเข้ามาสะสมครบเกณฑ์ (true/false)'
ON CONFLICT (key) DO NOTHING;

INSERT INTO system_configs (key, value, description)
SELECT 'line_sell_accum_threshold_gold',
       COALESCE(NULLIF((SELECT value FROM system_configs WHERE key = 'line_pending_sell_threshold_gold'), ''), '65'),
       'LINE: น้ำหนักทองสะสม (บาท) ต่อ 1 กิโลกรัมที่จะแจ้งขาย (0 = ปิด)'
ON CONFLICT (key) DO NOTHING;

INSERT INTO system_configs (key, value, description)
SELECT 'line_sell_accum_threshold_silver',
       COALESCE(NULLIF((SELECT value FROM system_configs WHERE key = 'line_pending_sell_threshold_silver'), ''), '1000'),
       'LINE: น้ำหนักเงินสะสม (กรัม) ต่อ 1 กิโลกรัมที่จะแจ้งขาย (0 = ปิด)'
ON CONFLICT (key) DO NOTHING;

INSERT INTO system_configs (key, value, description)
SELECT 'line_sell_accum_purity_gold',
       COALESCE(NULLIF((SELECT value FROM system_configs WHERE key = 'line_pending_sell_purity_gold'), ''), '99.99'),
       'LINE: ความบริสุทธิ์ (%) ที่ระบุในข้อความแจ้งขายทอง'
ON CONFLICT (key) DO NOTHING;

INSERT INTO system_configs (key, value, description)
SELECT 'line_sell_accum_purity_silver',
       COALESCE(NULLIF((SELECT value FROM system_configs WHERE key = 'line_pending_sell_purity_silver'), ''), '99.9'),
       'LINE: ความบริสุทธิ์ (%) ที่ระบุในข้อความแจ้งขายเงิน'
ON CONFLICT (key) DO NOTHING;

INSERT INTO system_configs (key, value, description) VALUES
  ('line_sell_accum_weight_gold',   '0', 'LINE: ภายใน — น้ำหนักทองสะสม (บาท) ที่ยังไม่ครบเกณฑ์'),
  ('line_sell_accum_weight_silver', '0', 'LINE: ภายใน — น้ำหนักเงินสะสม (กรัม) ที่ยังไม่ครบเกณฑ์')
ON CONFLICT (key) DO NOTHING;

DELETE FROM system_configs WHERE key LIKE 'line_pending_sell_%';

DROP TABLE IF EXISTS line_pending_sell_alerts;
