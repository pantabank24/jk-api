-- Custom-weight schedule: lets the shop pre-define, per weekday or per
-- datetime range, whether customers may type the bill weight directly
-- instead of using the fixed +/-5 baht stepper.
--
-- Resolution precedence: range rule (covering now) > weekday rule > no rule
-- (not allowed by default). The master switch custom_weight_enabled gates
-- everything.

CREATE TABLE IF NOT EXISTS custom_weight_schedules (
  id          BIGSERIAL PRIMARY KEY,
  scope       VARCHAR(10) NOT NULL,           -- 'weekday' | 'range'
  weekday     INT,                            -- 0=Sun .. 6=Sat (scope='weekday')
  start_at    TIMESTAMPTZ,                    -- range start (scope='range')
  end_at      TIMESTAMPTZ,                    -- range end (scope='range')
  enabled     BOOLEAN NOT NULL DEFAULT TRUE,  -- allow typing weight in this slot?
  open_time   VARCHAR(5) NOT NULL DEFAULT '09:30',
  close_time  VARCHAR(5) NOT NULL DEFAULT '16:30',
  note        VARCHAR(255) NOT NULL DEFAULT '',
  created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- At most one rule per weekday.
CREATE UNIQUE INDEX IF NOT EXISTS uq_custom_weight_schedules_weekday
  ON custom_weight_schedules(weekday) WHERE scope = 'weekday';

-- Ranges may overlap (latest start wins), so a plain index, not unique.
CREATE INDEX IF NOT EXISTS idx_custom_weight_schedules_range
  ON custom_weight_schedules(start_at, end_at) WHERE scope = 'range';

INSERT INTO system_configs (key, value, description) VALUES
  ('custom_weight_enabled', 'false', 'เปิด/ปิดการอนุญาตให้ลูกค้าพิมพ์น้ำหนักเอง (true/false)')
ON CONFLICT (key) DO NOTHING;
