-- Realtime cutoff: how late realtime-price selling stays open after the
-- association window. Empty = no cutoff (sell all night, previous behavior).

ALTER TABLE sales_schedules ADD COLUMN IF NOT EXISTS realtime_until VARCHAR(5) NOT NULL DEFAULT '';

INSERT INTO system_configs (key, value, description) VALUES
  ('sales_realtime_until', '', 'เวลาปิดขายโหมดเรียลไทม์ (HH:MM, ว่าง = ไม่จำกัด)')
ON CONFLICT (key) DO NOTHING;
