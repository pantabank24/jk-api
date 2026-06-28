-- Restore the previous employee grants.
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM roles r CROSS JOIN permissions p
WHERE r.name = 'employee' AND p.code IN ('members.read', 'credits.read', 'stores.read')
ON CONFLICT (role_id, permission_id) DO NOTHING;
