ALTER TABLE sales_schedules DROP COLUMN IF EXISTS realtime_only;

DELETE FROM system_configs WHERE key = 'sales_realtime_only';
