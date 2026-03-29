DELETE FROM role_permissions
WHERE permission_id IN (
    SELECT id FROM permissions WHERE code IN ('logs.read', 'logs.delete')
);

DELETE FROM permissions WHERE code IN ('logs.read', 'logs.delete');
