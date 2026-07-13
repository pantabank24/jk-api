DELETE FROM role_permissions WHERE permission_id IN (
    SELECT id FROM permissions WHERE code IN ('banks.read', 'banks.create', 'banks.update', 'banks.delete')
);
DELETE FROM permissions WHERE code IN ('banks.read', 'banks.create', 'banks.update', 'banks.delete');
DROP TABLE IF EXISTS banks;
