CREATE TABLE IF NOT EXISTS system_configs (
    id          BIGSERIAL PRIMARY KEY,
    key         VARCHAR(100) UNIQUE NOT NULL,
    value       TEXT         DEFAULT '',
    description VARCHAR(500) DEFAULT '',
    created_at  TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at  TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

INSERT INTO system_configs (key, value, description) VALUES
  ('gold_price_auto_fetch',  'true',         'เปิด/ปิดการดึงราคาทองอัตโนมัติ (true/false)'),
  ('gold_price_cron',        '*/30 * * * *', 'Cron expression สำหรับดึงราคาทองอัตโนมัติ')
ON CONFLICT (key) DO NOTHING;

INSERT INTO permissions (code, name, group_name, description)
VALUES
  ('config.read',   'ดู Config',     'config', 'ดูค่า system config'),
  ('config.update', 'แก้ไข Config',  'config', 'แก้ไขค่า system config')
ON CONFLICT (code) DO NOTHING;

INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM roles r, permissions p
WHERE r.name = 'master'
  AND p.code IN ('config.read','config.update')
ON CONFLICT DO NOTHING;
