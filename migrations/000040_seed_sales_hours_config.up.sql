-- Sale hours: quotations and bills can only be created while sales are open.
-- Default window 09:30-16:30 (Asia/Bangkok). Toggle via sales_hours_enabled.
INSERT INTO system_configs (key, value, description) VALUES
  ('sales_hours_enabled', 'true',  'เปิด/ปิดระบบจำกัดเวลาทำการขาย (true/false)'),
  ('sales_open_time',     '09:30', 'เวลาเปิดการขาย (HH:MM, เขตเวลาไทย)'),
  ('sales_close_time',    '16:30', 'เวลาปิดการขาย (HH:MM, เขตเวลาไทย)')
ON CONFLICT (key) DO NOTHING;
