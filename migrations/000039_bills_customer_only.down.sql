-- Restore the previous grants: master can create bills; owner/employee can
-- read + approve bills at the storefront.
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM roles r CROSS JOIN permissions p
WHERE r.name = 'master' AND p.code = 'bills.create'
ON CONFLICT (role_id, permission_id) DO NOTHING;

INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM roles r CROSS JOIN permissions p
WHERE r.name IN ('owner', 'employee') AND p.code IN ('bills.read', 'bills.approve')
ON CONFLICT (role_id, permission_id) DO NOTHING;
