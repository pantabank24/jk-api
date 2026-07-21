-- Realtime-only mode: sell at the live (TradingView) gold price around the
-- clock, ignoring the association window entirely. Overrides open/close time,
-- realtime_after_hours and realtime_until for the rule it is set on.

ALTER TABLE sales_schedules ADD COLUMN IF NOT EXISTS realtime_only BOOLEAN NOT NULL DEFAULT FALSE;

INSERT INTO system_configs (key, value, description) VALUES
  ('sales_realtime_only', 'false', 'ค่าเริ่มต้น: ขายราคาเรียลไทม์อย่างเดียวตลอด 24 ชม. (true/false)')
ON CONFLICT (key) DO NOTHING;
