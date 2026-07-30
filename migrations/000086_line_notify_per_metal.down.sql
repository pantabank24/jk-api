DELETE FROM system_configs WHERE key IN (
  'line_notify_gold_enabled',
  'line_notify_silver_enabled',
  'line_bill_notify_threshold_gold',
  'line_bill_notify_threshold_silver',
  'line_notify_latch_gold',
  'line_notify_latch_silver'
);
