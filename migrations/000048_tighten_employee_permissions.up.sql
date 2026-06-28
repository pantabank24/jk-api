-- Tighten the employee role: employees no longer see members, credit management,
-- or stores & branches. Reconciles databases seeded before this change.
DELETE FROM role_permissions
WHERE role_id IN (SELECT id FROM roles WHERE name = 'employee')
  AND permission_id IN (SELECT id FROM permissions WHERE code IN (
    'members.read', 'credits.read', 'stores.read'
  ));
