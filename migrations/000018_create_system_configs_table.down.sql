DELETE FROM role_permissions WHERE permission_id IN (SELECT id FROM permissions WHERE code IN ('config.read','config.update'));
DELETE FROM permissions WHERE code IN ('config.read','config.update');
DROP TABLE IF EXISTS system_configs;
