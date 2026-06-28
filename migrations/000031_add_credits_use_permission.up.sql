-- Add credits.use permission: roles with this permission have their quotations subject to credit deduction
INSERT INTO permissions (code, name, group_name, description)
VALUES ('credits.use', 'ใช้เครดิต', 'credits', 'ใบเสนอราคาของสิทธิ์นี้จะถูกหักเครดิต')
ON CONFLICT (code) DO NOTHING;

-- Assign to owner and employee (their quotations are required to spend credits)
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM roles r CROSS JOIN permissions p
WHERE r.name IN ('owner', 'employee') AND p.code = 'credits.use'
ON CONFLICT (role_id, permission_id) DO NOTHING;
