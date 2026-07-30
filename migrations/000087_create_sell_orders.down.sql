DELETE FROM role_permissions
WHERE permission_id IN (SELECT id FROM permissions WHERE code IN
  ('sell_orders.read', 'sell_orders.create', 'sell_orders.manage'));

DELETE FROM permissions WHERE code IN
  ('sell_orders.read', 'sell_orders.create', 'sell_orders.manage');

DELETE FROM system_configs WHERE key IN (
  'auto_sell_enabled', 'auto_sell_ignore_hours', 'auto_sell_max_slippage_thb',
  'auto_sell_max_active_orders', 'auto_sell_max_active_weight',
  'auto_sell_max_feed_age_sec', 'auto_sell_tick_seconds'
);

DROP INDEX IF EXISTS idx_quotations_sell_order_id;
DROP INDEX IF EXISTS idx_quotations_auto_sell;
ALTER TABLE quotations DROP COLUMN IF EXISTS sell_order_id;
ALTER TABLE quotations DROP COLUMN IF EXISTS auto_sell;

DROP TABLE IF EXISTS sell_orders;
