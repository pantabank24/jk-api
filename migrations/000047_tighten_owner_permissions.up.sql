-- Tighten the owner role: owners no longer see credit management, role
-- management, and cannot create/edit/delete branches (stores & branches become
-- read-only for them). Reconciles databases seeded before this change.
DELETE FROM role_permissions
WHERE role_id IN (SELECT id FROM roles WHERE name = 'owner')
  AND permission_id IN (SELECT id FROM permissions WHERE code IN (
    'credits.read', 'credits.update', 'roles.read',
    'branches.create', 'branches.update', 'branches.delete'
  ));
