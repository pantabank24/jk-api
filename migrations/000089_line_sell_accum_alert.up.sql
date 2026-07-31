-- LINE alert #2: announce a sale every time customers' sell-ins add up to a
-- full lot.
--
-- Unrelated to the บิลค้างเคลียร์ alert (migration 86): that one watches a pile
-- that goes up and down, this one is a running meter that never goes backwards.
-- Gold the shop bought is accumulated at ออกบิล (สถานะ 11 รอตรวจบิล) across every
-- customer of the shop; each full 65-บาท lot — the shop's 1-กิโลกรัม unit — is
-- announced once and subtracted from the meter, so a 130-บาท day announces two
-- lots and carries the remainder into tomorrow instead of re-announcing it.
--
-- Shares line_notify_enabled (master switch) and line_notify_target_id with the
-- backlog alert: same LINE destination, separate on/off.

INSERT INTO system_configs (key, value, description) VALUES
  ('line_sell_accum_enabled',   'false', 'LINE: แจ้งเตือนเมื่อยอดทองที่ลูกค้าขายเข้ามาสะสมครบเกณฑ์ (true/false)'),
  ('line_sell_accum_threshold', '65',    'LINE: น้ำหนักสะสม (บาท) ต่อ 1 กิโลกรัมที่จะแจ้งขาย (0 = ปิด)'),
  ('line_sell_accum_purity',    '99.99', 'LINE: ความบริสุทธิ์ (%) ที่ระบุในข้อความแจ้งขาย'),
  ('line_sell_accum_weight',    '0',     'LINE: ภายใน — น้ำหนักสะสม (บาท) ที่ยังไม่ครบเกณฑ์')
ON CONFLICT (key) DO NOTHING;
