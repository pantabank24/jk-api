-- sales.bypass lets a role create quotations even when sales are closed
-- (outside the association/realtime window). Master already bypasses in code;
-- this grants the same to owner and employee so staff can always quote.
INSERT INTO permissions (code, name, group_name, description) VALUES
    ('sales.bypass', 'ออกใบเสนอราคาได้ตลอดเวลา', 'sales', 'ออกใบเสนอราคาได้แม้อยู่นอกเวลาทำการ')
ON CONFLICT (code) DO NOTHING;

INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM roles r CROSS JOIN permissions p
WHERE r.name IN ('owner', 'employee') AND p.code = 'sales.bypass'
ON CONFLICT (role_id, permission_id) DO NOTHING;
