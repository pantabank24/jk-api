-- Owner quotations are now subject to credit deduction (previously only employees).
-- For databases already migrated past 000031, grant credits.use to the owner role.
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM roles r CROSS JOIN permissions p
WHERE r.name = 'owner' AND p.code = 'credits.use'
ON CONFLICT (role_id, permission_id) DO NOTHING;
