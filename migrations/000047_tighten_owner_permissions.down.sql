-- Restore the previous owner grants.
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM roles r CROSS JOIN permissions p
WHERE r.name = 'owner' AND p.code IN (
    'credits.read', 'credits.update', 'roles.read',
    'branches.create', 'branches.update', 'branches.delete'
)
ON CONFLICT (role_id, permission_id) DO NOTHING;
