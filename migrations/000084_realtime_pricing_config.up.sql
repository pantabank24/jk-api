-- Real-time pricing policy, moved out of the tv-price-svc sidecar so the shop
-- can retune it from Settings → ราคาและเวลาขาย instead of shipping a deploy.
--
-- The sidecar now only reports spot + USD/THB; jk-api turns those into the
-- shop's quote:
--   กลาง   = spot × USDTHB × 0.472951   (15.244/31.1035 × 96.5%, a physical
--                                        constant — deliberately NOT a config)
--   รับซื้อ = กลาง + premium - spread/2
--   ขายออก = กลาง + premium + spread/2
--
-- Seeded with the values running in production at the time of this migration.

INSERT INTO system_configs (key, value, description) VALUES
  ('realtime_premium_thb', '-20', 'ราคาเรียลไทม์: ค่าปรับจากราคากลาง (บาท) ติดลบ = ต่ำกว่าราคากลาง'),
  ('realtime_spread_thb',  '80',  'ราคาเรียลไทม์: ส่วนต่างระหว่างราคารับซื้อกับราคาขายออก (บาท)')
ON CONFLICT (key) DO NOTHING;
