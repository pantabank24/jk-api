DELETE FROM system_configs WHERE key = 'sales_realtime_until';
ALTER TABLE sales_schedules DROP COLUMN IF EXISTS realtime_until;
