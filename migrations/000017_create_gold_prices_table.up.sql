CREATE TABLE IF NOT EXISTS gold_prices (
    id                BIGSERIAL PRIMARY KEY,
    bar_buy           DECIMAL(12,2) DEFAULT 0,
    bar_sell          DECIMAL(12,2) DEFAULT 0,
    ornament_buy      DECIMAL(12,2) DEFAULT 0,
    ornament_sell     DECIMAL(12,2) DEFAULT 0,
    change_today      DECIMAL(10,2) DEFAULT 0,
    change_yesterday  DECIMAL(10,2) DEFAULT 0,
    gold_date         VARCHAR(100)  DEFAULT '',
    gold_time         VARCHAR(50)   DEFAULT '',
    gold_round        VARCHAR(50)   DEFAULT '',
    created_at        TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

INSERT INTO permissions (code, name, group_name, description)
VALUES
  ('gold_prices.read',   'ดูราคาทอง',     'gold', 'ดูราคาทองปัจจุบันและประวัติ'),
  ('gold_prices.create', 'ดึงราคาทอง',    'gold', 'ดึง/บันทึกราคาทองใหม่')
ON CONFLICT (code) DO NOTHING;

INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM roles r, permissions p
WHERE r.name = 'master'
  AND p.code IN ('gold_prices.read','gold_prices.create')
ON CONFLICT DO NOTHING;
