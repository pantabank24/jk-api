ALTER TABLE sales_schedules ADD COLUMN IF NOT EXISTS specific_date DATE;

UPDATE sales_schedules
   SET specific_date = start_at::date,
       scope         = 'date'
 WHERE scope = 'range' AND start_at IS NOT NULL;

DROP INDEX IF EXISTS idx_sales_schedules_range;
ALTER TABLE sales_schedules DROP COLUMN IF EXISTS start_at;
ALTER TABLE sales_schedules DROP COLUMN IF EXISTS end_at;

CREATE UNIQUE INDEX IF NOT EXISTS uq_sales_schedules_date
  ON sales_schedules(specific_date) WHERE scope = 'date';
