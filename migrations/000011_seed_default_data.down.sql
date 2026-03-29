-- Remove seeded master user
DELETE FROM users WHERE email = 'admin@jk.com';

-- Remove role_permissions
DELETE FROM role_permissions;

-- Remove permissions
DELETE FROM permissions;

-- Remove roles
DELETE FROM roles WHERE is_system = TRUE;
