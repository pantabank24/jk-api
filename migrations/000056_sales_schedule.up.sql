-- Sales schedule: lets the shop pre-define, per weekday or per specific date,
-- whether selling is open, the association trading window, and whether real-time
-- gold pricing kicks in outside that window. (000057 later widens specific_date
-- into a datetime range.)
--
-- Resolution precedence: specific-date/range rule > weekday rule > the default
-- config keys. The master switch sales_enabled gates everything.

CREATE TABLE IF NOT EXISTS sales_schedules (
  id                    BIGSERIAL PRIMARY KEY,
  scope                 VARCHAR(10) NOT NULL,            -- 'weekday' | 'date'
  weekday               INT,                             -- 0=Sun .. 6=Sat (scope='weekday')
  specific_date         DATE,                            -- (scope='date')
  enabled               BOOLEAN NOT NULL DEFAULT TRUE,   -- can sell on this day?
  open_time             VARCHAR(5) NOT NULL DEFAULT '09:30',
  close_time            VARCHAR(5) NOT NULL DEFAULT '16:30',
  realtime_after_hours  BOOLEAN NOT NULL DEFAULT FALSE,  -- use real-time price outside the window
  note                  VARCHAR(255) NOT NULL DEFAULT '',
  created_at            TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at            TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- At most one rule per weekday and one per specific date.
CREATE UNIQUE INDEX IF NOT EXISTS uq_sales_schedules_weekday
  ON sales_schedules(weekday) WHERE scope = 'weekday';
CREATE UNIQUE INDEX IF NOT EXISTS uq_sales_schedules_date
  ON sales_schedules(specific_date) WHERE scope = 'date';

-- New config keys for the sales-price settings page.
INSERT INTO system_configs (key, value, description) VALUES
  ('sales_enabled',              'true',  'เปิด/ปิดการขายและออกใบเสนอราคาทั้งระบบ (true/false)'),
  ('sales_realtime_after_hours', 'false', 'ค่าเริ่มต้น: ใช้ราคาเรียลไทม์เมื่อหมดเวลาสมาคม (true/false)')
ON CONFLICT (key) DO NOTHING;
