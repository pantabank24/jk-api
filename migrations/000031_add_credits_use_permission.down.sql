DELETE FROM role_permissions WHERE permission_id = (SELECT id FROM permissions WHERE code = 'credits.use');
DELETE FROM permissions WHERE code = 'credits.use';
