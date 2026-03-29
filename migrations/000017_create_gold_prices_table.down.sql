DELETE FROM role_permissions WHERE permission_id IN (SELECT id FROM permissions WHERE code LIKE 'gold_prices.%');
DELETE FROM permissions WHERE code LIKE 'gold_prices.%';
DROP TABLE IF EXISTS gold_prices;
