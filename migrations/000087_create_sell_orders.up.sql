-- Auto-sell orders: a customer pre-commits to selling a fixed weight once the
-- real-time buy price reaches their target, like a stock limit order. The engine
-- (internal/service/sell_order_engine.go) polls the live price and fills them.
--
-- The target is compared against the SAME number the customer sees on the sell
-- screen — bar_buy, i.e. after realtime_premium_thb and realtime_spread_thb.
-- premium_at_create/spread_at_create snapshot that policy so a later retune can
-- be explained, but the fill always prices at the CURRENT policy: an order must
-- never fill at a price the shop no longer quotes.

CREATE TABLE IF NOT EXISTS sell_orders (
  id           BIGSERIAL PRIMARY KEY,
  user_id      BIGINT NOT NULL,                 -- the customer the bill lands on
  created_by   BIGINT,                          -- who placed it (staff on behalf, else = user_id)
  store_id     BIGINT,
  branch_id    BIGINT,
  metal        VARCHAR(20) NOT NULL DEFAULT 'gold',
  gold_type_id BIGINT,                          -- gold type used for pricing (ทองคำแท่ง 96.5%)
  type_name    VARCHAR(100) NOT NULL DEFAULT '',
  weight       NUMERIC(10,4) NOT NULL,          -- น้ำหนัก (บาททอง), same unit as the sell screen
  target_price NUMERIC(12,2) NOT NULL,          -- fill when bar_buy >= this

  -- active  → waiting for the price
  -- filling → engine claimed it this tick (transient; recovered on boot)
  -- filled  → a bill was created
  -- cancelled → customer/staff cancelled it
  status       VARCHAR(12) NOT NULL DEFAULT 'active',

  -- Pricing policy + market price at the moment the order was placed (audit only).
  premium_at_create NUMERIC(10,2) NOT NULL DEFAULT 0,
  spread_at_create  NUMERIC(10,2) NOT NULL DEFAULT 0,
  price_at_create   NUMERIC(12,2) NOT NULL DEFAULT 0,

  -- Fill result. filled_price is the price actually captured, which may sit above
  -- target_price when the market jumped between ticks — both are kept so the
  -- difference is always explainable.
  filled_price NUMERIC(12,2),
  filled_at    TIMESTAMPTZ,
  bill_id      BIGINT,

  cancelled_at  TIMESTAMPTZ,
  cancel_reason VARCHAR(255) NOT NULL DEFAULT '',
  note          VARCHAR(255) NOT NULL DEFAULT '',
  created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- The engine's hot query: active orders whose target the price has reached.
CREATE INDEX IF NOT EXISTS idx_sell_orders_due
  ON sell_orders(target_price) WHERE status = 'active';
CREATE INDEX IF NOT EXISTS idx_sell_orders_user ON sell_orders(user_id, status);

-- Mark the bills the engine creates so they are distinguishable from a bill the
-- customer submitted by hand (chip in the list + filter).
ALTER TABLE quotations ADD COLUMN IF NOT EXISTS auto_sell     BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE quotations ADD COLUMN IF NOT EXISTS sell_order_id BIGINT;
CREATE INDEX IF NOT EXISTS idx_quotations_auto_sell ON quotations(auto_sell) WHERE auto_sell;
-- Used by the boot recovery to tell "bill created" from "crashed before it was".
CREATE INDEX IF NOT EXISTS idx_quotations_sell_order_id ON quotations(sell_order_id);

INSERT INTO system_configs (key, value, description) VALUES
  ('auto_sell_enabled',           'false', 'เปิด/ปิดระบบตั้งราคาขายอัตโนมัติทั้งระบบ (true/false)'),
  ('auto_sell_ignore_hours',      'true',  'ยิงคำสั่งขายได้แม้อยู่นอกเวลาทำการ (true/false)'),
  ('auto_sell_max_slippage_thb',  '0',     'ส่วนต่างสูงสุดที่ยอมให้ราคาเกินเป้า (บาท, 0 = ไม่จำกัด) กันราคาเพี้ยนยิงผิด'),
  ('auto_sell_max_active_orders', '5',     'จำนวนคำสั่งขายที่รออยู่ได้ต่อลูกค้า 1 คน'),
  ('auto_sell_max_active_weight', '200',   'น้ำหนักรวมของคำสั่งขายที่รออยู่ได้ต่อลูกค้า 1 คน (บาททอง)'),
  ('auto_sell_max_feed_age_sec',  '15',    'อายุราคาเรียลไทม์สูงสุดที่ยอมให้ยิงได้ (วินาที) เกินนี้ถือว่าราคาค้าง'),
  ('auto_sell_tick_seconds',      '5',     'ความถี่ในการตรวจราคาเทียบคำสั่งขาย (วินาที)')
ON CONFLICT (key) DO NOTHING;

INSERT INTO permissions (code, name, group_name, description) VALUES
  ('sell_orders.read',   'ดูคำสั่งขายอัตโนมัติ',   'sell_orders', 'ดูรายการคำสั่งขายอัตโนมัติ'),
  ('sell_orders.create', 'ตั้งคำสั่งขายอัตโนมัติ', 'sell_orders', 'ลูกค้าตั้งราคาเป้าหมายเพื่อขายอัตโนมัติ'),
  ('sell_orders.manage', 'จัดการคำสั่งขายอัตโนมัติ', 'sell_orders', 'ดู/ยกเลิกคำสั่งขายอัตโนมัติของลูกค้าทุกคน')
ON CONFLICT (code) DO NOTHING;

-- Master manages every customer's orders but does not place them (mirrors bills).
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM roles r CROSS JOIN permissions p
WHERE r.name = 'master' AND p.code IN ('sell_orders.read', 'sell_orders.manage')
ON CONFLICT (role_id, permission_id) DO NOTHING;

-- Customers place and cancel their own.
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM roles r CROSS JOIN permissions p
WHERE r.name = 'customer' AND p.code IN ('sell_orders.read', 'sell_orders.create')
ON CONFLICT (role_id, permission_id) DO NOTHING;
