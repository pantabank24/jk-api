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

-- owner: no credit-management menu (owners don't manage credits)

-- employee: no credit-management menu
