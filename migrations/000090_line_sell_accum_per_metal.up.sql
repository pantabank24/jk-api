-- Sell-in accumulation alert split per metal, for the same reason migration 86
-- split the backlog alert: bills are single-metal, ทอง and เงิน are weighed in
-- different units (บาท vs กรัม) and announced at different purities, so one
-- shared meter could never describe both.
--
-- Gold inherits whatever migration 89 left configured — including a part-filled
-- meter, which is real gold on the bench and must not be thrown away. Silver
-- starts from scratch, switched off, at 1000 กรัม per lot.

INSERT INTO system_configs (key, value, description)
SELECT 'line_sell_accum_gold_enabled',
       COALESCE((SELECT value FROM system_configs WHERE key = 'line_sell_accum_enabled'), 'false'),
       'LINE: แจ้งเตือนเมื่อยอดทองที่ลูกค้าขายเข้ามาสะสมครบเกณฑ์ (true/false)'
ON CONFLICT (key) DO NOTHING;

INSERT INTO system_configs (key, value, description) VALUES
  ('line_sell_accum_silver_enabled', 'false', 'LINE: แจ้งเตือนเมื่อยอดเงินที่ลูกค้าขายเข้ามาสะสมครบเกณฑ์ (true/false)')
ON CONFLICT (key) DO NOTHING;

INSERT INTO system_configs (key, value, description)
SELECT 'line_sell_accum_threshold_gold',
       COALESCE(NULLIF((SELECT value FROM system_configs WHERE key = 'line_sell_accum_threshold'), ''), '65'),
       'LINE: น้ำหนักทองสะสม (บาท) ต่อ 1 กิโลกรัมที่จะแจ้งขาย (0 = ปิด)'
ON CONFLICT (key) DO NOTHING;

INSERT INTO system_configs (key, value, description) VALUES
  ('line_sell_accum_threshold_silver', '1000', 'LINE: น้ำหนักเงินสะสม (กรัม) ต่อ 1 กิโลกรัมที่จะแจ้งขาย (0 = ปิด)')
ON CONFLICT (key) DO NOTHING;

INSERT INTO system_configs (key, value, description)
SELECT 'line_sell_accum_purity_gold',
       COALESCE(NULLIF((SELECT value FROM system_configs WHERE key = 'line_sell_accum_purity'), ''), '99.99'),
       'LINE: ความบริสุทธิ์ (%) ที่ระบุในข้อความแจ้งขายทอง'
ON CONFLICT (key) DO NOTHING;

INSERT INTO system_configs (key, value, description) VALUES
  ('line_sell_accum_purity_silver', '99.9', 'LINE: ความบริสุทธิ์ (%) ที่ระบุในข้อความแจ้งขายเงิน')
ON CONFLICT (key) DO NOTHING;

-- The meters themselves. Gold carries its running total across the split.
INSERT INTO system_configs (key, value, description)
SELECT 'line_sell_accum_weight_gold',
       COALESCE(NULLIF((SELECT value FROM system_configs WHERE key = 'line_sell_accum_weight'), ''), '0'),
       'LINE: ภายใน — น้ำหนักทองสะสม (บาท) ที่ยังไม่ครบเกณฑ์'
ON CONFLICT (key) DO NOTHING;

INSERT INTO system_configs (key, value, description) VALUES
  ('line_sell_accum_weight_silver', '0', 'LINE: ภายใน — น้ำหนักเงินสะสม (กรัม) ที่ยังไม่ครบเกณฑ์')
ON CONFLICT (key) DO NOTHING;

DELETE FROM system_configs WHERE key IN (
  'line_sell_accum_enabled',
  'line_sell_accum_threshold',
  'line_sell_accum_purity',
  'line_sell_accum_weight'
);
