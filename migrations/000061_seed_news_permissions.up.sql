INSERT INTO permissions (code, name, group_name, description) VALUES
    ('news.read',   'ดูข่าวสารทั้งหมด (จัดการ)', 'news', 'ดูรายการข่าวสารทั้งหมดสำหรับจัดการ'),
    ('news.create', 'สร้างข่าวสาร',             'news', 'สร้างข่าวสารใหม่'),
    ('news.update', 'แก้ไขข่าวสาร',             'news', 'แก้ไขข่าวสาร'),
    ('news.delete', 'ลบข่าวสาร',               'news', 'ลบข่าวสาร')
ON CONFLICT (code) DO NOTHING;

INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM roles r CROSS JOIN permissions p
WHERE r.name = 'master' AND p.code IN ('news.read', 'news.create', 'news.update', 'news.delete')
ON CONFLICT (role_id, permission_id) DO NOTHING;
