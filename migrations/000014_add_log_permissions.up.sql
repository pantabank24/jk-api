-- Add log permissions
INSERT INTO permissions (code, name, group_name, description)
VALUES
    ('logs.read',   'ดู Logs',   'logs', 'ดู activity logs และ login logs'),
    ('logs.delete', 'ลบ Logs',   'logs', 'ลบ logs เก่า')
ON CONFLICT (code) DO NOTHING;

-- Assign logs.read + logs.delete to master role
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM roles r, permissions p
WHERE r.name = 'master'
  AND p.code IN ('logs.read', 'logs.delete')
ON CONFLICT DO NOTHING;
