DELETE FROM role_permissions WHERE permission_id IN (
    SELECT id FROM permissions WHERE code IN ('credits.read', 'credits.update')
);
DELETE FROM permissions WHERE code IN ('credits.read', 'credits.update');
