-- Add credit management permissions
INSERT INTO permissions (code, name, group_name, description) VALUES
    ('credits.read',   'ดูรายการเครดิต',    'credits', 'สามารถดูประวัติการเคลื่อนไหวเครดิต'),
    ('credits.update', 'จัดการเครดิต',      'credits', 'สามารถเติม/ลดเครดิตสมาชิก')
ON CONFLICT (code) DO NOTHING;

-- master: auto-assigned via CROSS JOIN below
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM roles r CROSS JOIN permissions p
WHERE r.name = 'master' AND p.code IN ('credits.read', 'credits.update')
ON CONFLICT (role_id, permission_id) DO NOTHING;

-- owner
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM roles r CROSS JOIN permissions p
WHERE r.name = 'owner' AND p.code IN ('credits.read', 'credits.update')
ON CONFLICT (role_id, permission_id) DO NOTHING;

-- branch
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM roles r CROSS JOIN permissions p
WHERE r.name = 'branch' AND p.code IN ('credits.read', 'credits.update')
ON CONFLICT (role_id, permission_id) DO NOTHING;

-- employee: read only
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM roles r CROSS JOIN permissions p
WHERE r.name = 'employee' AND p.code IN ('credits.read')
ON CONFLICT (role_id, permission_id) DO NOTHING;
