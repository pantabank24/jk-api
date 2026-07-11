-- bills.sell lets staff (master/owner/employee) create a bill on behalf of a
-- chosen customer. It is distinct from bills.create (the customer self-service
-- flow) so staff keep their own dashboards while gaining a "sell for a customer"
-- action, and so the two flows can be gated independently.
INSERT INTO permissions (code, name, group_name, description) VALUES
    ('bills.sell', 'ขายแทนลูกค้า', 'bills', 'เลือกลูกค้าแล้วทำรายการขายแทนลูกค้าได้')
ON CONFLICT (code) DO NOTHING;

-- Grant to master (which does NOT auto-receive permissions added after the
-- initial seed), owner, and employee.
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM roles r CROSS JOIN permissions p
WHERE r.name IN ('master', 'owner', 'employee') AND p.code = 'bills.sell'
ON CONFLICT (role_id, permission_id) DO NOTHING;
