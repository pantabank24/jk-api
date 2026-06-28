-- Revoke credits.use from the owner role.
DELETE FROM role_permissions
WHERE role_id = (SELECT id FROM roles WHERE name = 'owner')
  AND permission_id = (SELECT id FROM permissions WHERE code = 'credits.use');
