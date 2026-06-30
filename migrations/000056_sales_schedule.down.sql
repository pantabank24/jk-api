DELETE FROM system_configs WHERE key IN ('sales_enabled', 'sales_realtime_after_hours');
DROP TABLE IF EXISTS sales_schedules;
