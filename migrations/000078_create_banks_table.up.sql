-- Banks a customer can be paid into. Kept as its own table (rather than a hard-coded
-- enum) so the shop can add/disable banks without a deploy.
CREATE TABLE IF NOT EXISTS banks (
    id          SERIAL PRIMARY KEY,
    name        VARCHAR(100) NOT NULL,
    code        VARCHAR(20)  NOT NULL DEFAULT '',
    sort_order  INT          NOT NULL DEFAULT 0,
    is_active   BOOLEAN      NOT NULL DEFAULT TRUE,
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

-- The bank list itself is seeded separately (000080), so re-seeding data and
-- changing the schema stay independent.

INSERT INTO permissions (code, name, group_name, description) VALUES
    ('banks.read',   'ดูธนาคาร',     'banks', 'ดูรายการธนาคาร'),
    ('banks.create', 'เพิ่มธนาคาร',  'banks', 'เพิ่มธนาคารใหม่'),
    ('banks.update', 'แก้ไขธนาคาร',  'banks', 'แก้ไขข้อมูลธนาคาร'),
    ('banks.delete', 'ลบธนาคาร',     'banks', 'ลบธนาคาร')
ON CONFLICT (code) DO NOTHING;

-- master does NOT auto-receive permissions added after the initial seed, so grant
-- explicitly. Managing the bank list is an owner/master job; employees only read.
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM roles r CROSS JOIN permissions p
WHERE r.name IN ('master', 'owner') AND p.code IN ('banks.read', 'banks.create', 'banks.update', 'banks.delete')
ON CONFLICT (role_id, permission_id) DO NOTHING;

INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM roles r CROSS JOIN permissions p
WHERE r.name = 'employee' AND p.code = 'banks.read'
ON CONFLICT (role_id, permission_id) DO NOTHING;
