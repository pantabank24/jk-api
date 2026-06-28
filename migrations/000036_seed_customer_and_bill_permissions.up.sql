-- Seed the customer role
INSERT INTO roles (name, display_name, description, is_system) VALUES
    ('customer', 'ลูกค้า', 'ลูกค้าของร้าน สามารถออกบิลและดูบิลของตัวเองได้', TRUE)
ON CONFLICT (name) DO NOTHING;

-- Seed bill + customer permissions
INSERT INTO permissions (code, name, group_name, description) VALUES
    -- Bills (the customer-facing name for quotations)
    ('bills.create',  'สร้างบิล',        'bills',     'ลูกค้าสามารถสร้างบิลใหม่'),
    ('bills.read',    'ดูบิล',           'bills',     'สามารถดูบิล'),
    ('bills.issue',   'ออกบิล',          'bills',     'ออกบิลให้ลูกค้า (รอออกบิล → รอตรวจบิล)'),
    ('bills.approve', 'อนุมัติ/ยกเลิกบิล', 'bills',     'อนุมัติปิดบิลหรือยกเลิกบิล (รอตรวจบิล → สำเร็จ/ยกเลิก)'),
    -- Customers (managed by master)
    ('customers.create', 'สร้างลูกค้า', 'customers', 'สามารถเพิ่มลูกค้าใหม่'),
    ('customers.read',   'ดูลูกค้า',    'customers', 'สามารถดูข้อมูลลูกค้า'),
    ('customers.update', 'แก้ไขลูกค้า', 'customers', 'สามารถแก้ไขข้อมูลลูกค้า'),
    ('customers.delete', 'ลบลูกค้า',    'customers', 'สามารถลบลูกค้า')
ON CONFLICT (code) DO NOTHING;

-- Bills are a CUSTOMER-ONLY flow: customers create/view their own bills, and
-- master manages them (issue/approve/cancel + customer management). Master does
-- NOT create bills, and owner/employee have no bill access at all.
-- (master does NOT auto-receive permissions added after the initial seed, so
-- grant them explicitly.)
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM roles r CROSS JOIN permissions p
WHERE r.name = 'master' AND p.code IN (
    'bills.read', 'bills.issue', 'bills.approve',
    'customers.create', 'customers.read', 'customers.update', 'customers.delete'
)
ON CONFLICT (role_id, permission_id) DO NOTHING;

-- Customer role: create + view own bills only
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM roles r CROSS JOIN permissions p
WHERE r.name = 'customer' AND p.code IN ('bills.create', 'bills.read')
ON CONFLICT (role_id, permission_id) DO NOTHING;
