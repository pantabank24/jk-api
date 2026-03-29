-- Seed default roles
INSERT INTO roles (name, display_name, description, is_system) VALUES
    ('master', 'Master', 'ผู้ดูแลระบบสูงสุด มีสิทธิ์เข้าถึงทุกร้านและทุกฟังก์ชัน', TRUE),
    ('owner', 'เจ้าของร้าน', 'เจ้าของร้าน สามารถจัดการได้ทั้งร้านและสาขาในสังกัด', TRUE),
    ('branch', 'ผู้จัดการสาขา', 'ผู้จัดการสาขา สามารถจัดการได้เฉพาะสาขาของตน', TRUE),
    ('employee', 'พนักงาน', 'พนักงาน สามารถออกใบเสนอราคาและดูสมาชิกได้', TRUE)
ON CONFLICT (name) DO NOTHING;

-- Seed permissions
INSERT INTO permissions (code, name, group_name, description) VALUES
    -- Stores
    ('stores.create', 'สร้างร้านค้า', 'stores', 'สามารถสร้างร้านค้าใหม่'),
    ('stores.read', 'ดูร้านค้า', 'stores', 'สามารถดูข้อมูลร้านค้า'),
    ('stores.update', 'แก้ไขร้านค้า', 'stores', 'สามารถแก้ไขข้อมูลร้านค้า'),
    ('stores.delete', 'ลบร้านค้า', 'stores', 'สามารถลบร้านค้า'),
    -- Branches
    ('branches.create', 'สร้างสาขา', 'branches', 'สามารถสร้างสาขาใหม่'),
    ('branches.read', 'ดูสาขา', 'branches', 'สามารถดูข้อมูลสาขา'),
    ('branches.update', 'แก้ไขสาขา', 'branches', 'สามารถแก้ไขข้อมูลสาขา'),
    ('branches.delete', 'ลบสาขา', 'branches', 'สามารถลบสาขา'),
    -- Quotations
    ('quotations.create', 'สร้างใบเสนอราคา', 'quotations', 'สามารถสร้างใบเสนอราคาใหม่'),
    ('quotations.read', 'ดูใบเสนอราคา', 'quotations', 'สามารถดูใบเสนอราคา'),
    ('quotations.update', 'แก้ไขใบเสนอราคา', 'quotations', 'สามารถแก้ไข/อนุมัติ/ยกเลิกใบเสนอราคา'),
    ('quotations.delete', 'ลบใบเสนอราคา', 'quotations', 'สามารถลบใบเสนอราคา'),
    -- Members
    ('members.create', 'สร้างสมาชิก', 'members', 'สามารถเพิ่มสมาชิกใหม่'),
    ('members.read', 'ดูสมาชิก', 'members', 'สามารถดูข้อมูลสมาชิก'),
    ('members.update', 'แก้ไขสมาชิก', 'members', 'สามารถแก้ไขข้อมูลสมาชิก'),
    ('members.delete', 'ลบสมาชิก', 'members', 'สามารถลบสมาชิก'),
    -- Users
    ('users.create', 'สร้างผู้ใช้', 'users', 'สามารถสร้างผู้ใช้ใหม่'),
    ('users.read', 'ดูผู้ใช้', 'users', 'สามารถดูข้อมูลผู้ใช้'),
    ('users.update', 'แก้ไขผู้ใช้', 'users', 'สามารถแก้ไขข้อมูลผู้ใช้'),
    ('users.delete', 'ลบผู้ใช้', 'users', 'สามารถลบผู้ใช้'),
    -- Roles
    ('roles.create', 'สร้างสิทธิ์', 'roles', 'สามารถสร้างสิทธิ์ใหม่'),
    ('roles.read', 'ดูสิทธิ์', 'roles', 'สามารถดูข้อมูลสิทธิ์'),
    ('roles.update', 'แก้ไขสิทธิ์', 'roles', 'สามารถแก้ไขข้อมูลสิทธิ์'),
    ('roles.delete', 'ลบสิทธิ์', 'roles', 'สามารถลบสิทธิ์')
ON CONFLICT (code) DO NOTHING;

-- Assign ALL permissions to master role
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM roles r CROSS JOIN permissions p WHERE r.name = 'master'
ON CONFLICT (role_id, permission_id) DO NOTHING;

-- Assign owner permissions
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM roles r CROSS JOIN permissions p
WHERE r.name = 'owner' AND p.code IN (
    'stores.read',
    'branches.create', 'branches.read', 'branches.update', 'branches.delete',
    'quotations.create', 'quotations.read', 'quotations.update', 'quotations.delete',
    'members.create', 'members.read', 'members.update', 'members.delete',
    'users.create', 'users.read', 'users.update', 'users.delete',
    'roles.read'
)
ON CONFLICT (role_id, permission_id) DO NOTHING;

-- Assign branch permissions
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM roles r CROSS JOIN permissions p
WHERE r.name = 'branch' AND p.code IN (
    'stores.read',
    'branches.read',
    'quotations.create', 'quotations.read', 'quotations.update', 'quotations.delete',
    'members.create', 'members.read', 'members.update', 'members.delete',
    'users.read',
    'roles.read'
)
ON CONFLICT (role_id, permission_id) DO NOTHING;

-- Assign employee permissions
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM roles r CROSS JOIN permissions p
WHERE r.name = 'employee' AND p.code IN (
    'stores.read',
    'branches.read',
    'quotations.create', 'quotations.read',
    'members.read'
)
ON CONFLICT (role_id, permission_id) DO NOTHING;

-- Seed master user (password: Passw0rd@123!)
-- bcrypt hash of Passw0rd@123!
INSERT INTO users (name, email, password, role, is_active, role_id)
SELECT 'Admin', 'admin@jk.com', '$2a$10$bfB3xUFYYaZRLksHz8V1/.bzsL3YWv/lVaC.BCk9zZCkJvf4zrCjS', 'master', TRUE, r.id
FROM roles r WHERE r.name = 'master'
ON CONFLICT (email) DO NOTHING;
