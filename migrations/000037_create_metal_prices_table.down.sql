DELETE FROM role_permissions WHERE permission_id IN (
    SELECT id FROM permissions WHERE code IN ('metal_prices.read', 'metal_prices.create')
);
DELETE FROM permissions WHERE code IN ('metal_prices.read', 'metal_prices.create');
DROP TABLE IF EXISTS metal_prices;
