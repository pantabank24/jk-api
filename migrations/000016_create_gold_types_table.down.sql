DELETE FROM role_permissions WHERE permission_id IN (SELECT id FROM permissions WHERE code LIKE 'gold_types.%');
DELETE FROM permissions WHERE code LIKE 'gold_types.%';
DROP TABLE IF EXISTS gold_types;
