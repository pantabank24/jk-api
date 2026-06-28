-- Remove role-permission grants for the new permissions
DELETE FROM role_permissions
WHERE permission_id IN (
    SELECT id FROM permissions WHERE code IN (
        'bills.create', 'bills.read', 'bills.issue', 'bills.approve',
        'customers.create', 'customers.read', 'customers.update', 'customers.delete'
    )
);

-- Remove the permissions
DELETE FROM permissions WHERE code IN (
    'bills.create', 'bills.read', 'bills.issue', 'bills.approve',
    'customers.create', 'customers.read', 'customers.update', 'customers.delete'
);

-- Remove the customer role (and any of its remaining role_permissions)
DELETE FROM role_permissions WHERE role_id IN (SELECT id FROM roles WHERE name = 'customer');
DELETE FROM roles WHERE name = 'customer';
