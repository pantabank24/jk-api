DELETE FROM system_configs WHERE key IN (
  'line_sell_accum_enabled',
  'line_sell_accum_threshold',
  'line_sell_accum_purity',
  'line_sell_accum_weight'
);
