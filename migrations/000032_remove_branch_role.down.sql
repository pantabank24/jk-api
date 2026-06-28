-- Recreate the branch (ผู้จัดการสาขา) role and re-grant its permissions.
-- Note: users previously reassigned from branch to employee are NOT restored,
-- since that mapping is not reversible.
INSERT INTO roles (name, display_name, description, is_system) VALUES
    ('branch', 'ผู้จัดการสาขา', 'ผู้จัดการสาขา สามารถจัดการได้เฉพาะสาขาของตน', TRUE)
ON CONFLICT (name) DO NOTHING;

INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM roles r CROSS JOIN permissions p
WHERE r.name = 'branch' AND p.code IN (
    'stores.read',
    'branches.read',
    'quotations.create', 'quotations.read', 'quotations.update', 'quotations.delete',
    'members.create', 'members.read', 'members.update', 'members.delete',
    'users.read',
    'roles.read',
    'credits.read', 'credits.update'
)
ON CONFLICT (role_id, permission_id) DO NOTHING;
