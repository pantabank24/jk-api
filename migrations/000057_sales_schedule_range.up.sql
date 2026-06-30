-- Widen sales_schedules date rules into datetime ranges (start_at..end_at).

ALTER TABLE sales_schedules ADD COLUMN IF NOT EXISTS start_at TIMESTAMPTZ;
ALTER TABLE sales_schedules ADD COLUMN IF NOT EXISTS end_at   TIMESTAMPTZ;

-- Convert any existing single-date rules into a full-day range.
UPDATE sales_schedules
   SET start_at = specific_date::timestamptz,
       end_at   = (specific_date + INTERVAL '1 day' - INTERVAL '1 minute')::timestamptz,
       scope    = 'range'
 WHERE scope = 'date' AND specific_date IS NOT NULL;

DROP INDEX IF EXISTS uq_sales_schedules_date;
ALTER TABLE sales_schedules DROP COLUMN IF EXISTS specific_date;

-- Ranges may overlap (latest start wins), so a plain index, not unique.
CREATE INDEX IF NOT EXISTS idx_sales_schedules_range
  ON sales_schedules(start_at, end_at) WHERE scope = 'range';
