-- LINE bill-backlog alerts split per metal.
--
-- Bills are single-metal, and so is the เคลียร์บิล screen (gold and silver are
-- cleared separately) — so one shared threshold could never match what either
-- page shows. Each metal now carries its own toggle + threshold, and a latch
-- flag so the alert fires on the upward crossing only (19 → 20) instead of on
-- every approval while the backlog sits above the line.
--
-- line_notify_enabled stays as the master switch; the per-metal toggles seed
-- from it, and the per-metal thresholds from the old shared value, so an
-- already-configured shop keeps behaving the same after the migration.

INSERT INTO system_configs (key, value, description)
SELECT 'line_notify_gold_enabled',
       COALESCE((SELECT value FROM system_configs WHERE key = 'line_notify_enabled'), 'false'),
       'LINE: แจ้งเตือนบิลทองค้างเคลียร์ (true/false)'
ON CONFLICT (key) DO NOTHING;

INSERT INTO system_configs (key, value, description)
SELECT 'line_notify_silver_enabled',
       COALESCE((SELECT value FROM system_configs WHERE key = 'line_notify_enabled'), 'false'),
       'LINE: แจ้งเตือนบิลเงินค้างเคลียร์ (true/false)'
ON CONFLICT (key) DO NOTHING;

INSERT INTO system_configs (key, value, description)
SELECT 'line_bill_notify_threshold_gold',
       COALESCE(NULLIF((SELECT value FROM system_configs WHERE key = 'line_bill_notify_threshold'), ''), '5'),
       'LINE: จำนวนบิลทองค้างเคลียร์ที่จะแจ้งเตือน (0 = ปิด)'
ON CONFLICT (key) DO NOTHING;

INSERT INTO system_configs (key, value, description)
SELECT 'line_bill_notify_threshold_silver',
       COALESCE(NULLIF((SELECT value FROM system_configs WHERE key = 'line_bill_notify_threshold'), ''), '5'),
       'LINE: จำนวนบิลเงินค้างเคลียร์ที่จะแจ้งเตือน (0 = ปิด)'
ON CONFLICT (key) DO NOTHING;

-- Latch state: "true" = already alerted for this crossing, stays quiet until the
-- backlog drops back under the threshold. Managed by the API, not by the user.
INSERT INTO system_configs (key, value, description) VALUES
  ('line_notify_latch_gold',   'false', 'LINE: ภายใน — แจ้งเตือนบิลทองรอบนี้ไปแล้ว (กันส่งซ้ำ)'),
  ('line_notify_latch_silver', 'false', 'LINE: ภายใน — แจ้งเตือนบิลเงินรอบนี้ไปแล้ว (กันส่งซ้ำ)')
ON CONFLICT (key) DO NOTHING;
