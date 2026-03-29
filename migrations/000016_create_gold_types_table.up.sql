CREATE TABLE IF NOT EXISTS gold_types (
    id          BIGSERIAL PRIMARY KEY,
    name        VARCHAR(100) NOT NULL,
    description TEXT         DEFAULT '',
    price_source VARCHAR(30) DEFAULT 'bar_buy',
    default_percent DECIMAL(10,4) DEFAULT 0,
    default_plus    DECIMAL(12,2) DEFAULT 0,
    sort_order  INT          DEFAULT 0,
    is_active   BOOLEAN      DEFAULT true,
    created_at  TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at  TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- Initial gold types
INSERT INTO gold_types (name, description, price_source, default_percent, sort_order) VALUES
('ทองคำแท่ง',    'ทองคำแท่ง 99.99%',            'bar_buy',       99.99, 1),
('ทองรูปพรรณ',   'ทองรูปพรรณ 96.5%',             'ornament_buy',  96.50, 2),
('ทองเก่า',      'ทองเก่า / ทองบริสุทธิ์',         'bar_buy',       96.00, 3),
('เงิน',         'โลหะเงิน (Silver)',              'bar_buy',        0,    4),
('แพลตินัม',     'โลหะแพลตินัม (Platinum)',        'bar_buy',        0,    5),
('แพลเลเดียม',   'โลหะแพลเลเดียม (Palladium)',     'bar_buy',        0,    6);

-- Permissions
INSERT INTO permissions (code, name, group_name, description)
VALUES
  ('gold_types.read',   'ดูประเภททอง',       'gold',   'ดูรายการประเภททอง'),
  ('gold_types.create', 'เพิ่มประเภททอง',    'gold',   'เพิ่มประเภททอง'),
  ('gold_types.update', 'แก้ไขประเภททอง',   'gold',   'แก้ไขประเภททองและสูตรคำนวณ'),
  ('gold_types.delete', 'ลบประเภททอง',      'gold',   'ลบประเภททอง')
ON CONFLICT (code) DO NOTHING;

-- Assign all gold_types permissions to master role
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM roles r, permissions p
WHERE r.name = 'master'
  AND p.code IN ('gold_types.read','gold_types.create','gold_types.update','gold_types.delete')
ON CONFLICT DO NOTHING;
