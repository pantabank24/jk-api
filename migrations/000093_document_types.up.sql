-- Document types a customer document can be tagged with (บัตรประชาชน, เล่มบัญชี, ...).
-- Its own table rather than a hard-coded enum so the shop can add/disable types
-- without a deploy — same shape as banks (000078).
CREATE TABLE IF NOT EXISTS document_types (
    id          SERIAL PRIMARY KEY,
    name        VARCHAR(100) NOT NULL,
    code        VARCHAR(50)  NOT NULL DEFAULT '',
    sort_order  INT          NOT NULL DEFAULT 0,
    is_active   BOOLEAN      NOT NULL DEFAULT TRUE,
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

-- Existing documents predate the type, so the column is nullable and old rows
-- stay untyped ("ไม่ระบุประเภท") instead of being back-filled with a guess.
ALTER TABLE customer_documents
    ADD COLUMN IF NOT EXISTS document_type_id INT NULL
    REFERENCES document_types(id) ON DELETE SET NULL;

CREATE INDEX IF NOT EXISTS idx_customer_documents_document_type_id
    ON customer_documents (document_type_id);

INSERT INTO document_types (name, code, sort_order) VALUES
    ('บัตรประชาชน',            'id_card',       1),
    ('ทะเบียนบ้าน',            'house_reg',     2),
    ('เล่มบัญชีธนาคาร',        'bank_book',     3),
    ('หนังสือรับรองบริษัท',    'company_cert',  4),
    ('ภ.พ.20',                 'vat20',         5),
    ('อื่นๆ',                  'other',        99)
ON CONFLICT DO NOTHING;

INSERT INTO permissions (code, name, group_name, description) VALUES
    ('document_types.read',   'ดูประเภทเอกสาร',    'document_types', 'ดูรายการประเภทเอกสาร'),
    ('document_types.create', 'เพิ่มประเภทเอกสาร', 'document_types', 'เพิ่มประเภทเอกสารใหม่'),
    ('document_types.update', 'แก้ไขประเภทเอกสาร', 'document_types', 'แก้ไขประเภทเอกสาร'),
    ('document_types.delete', 'ลบประเภทเอกสาร',    'document_types', 'ลบประเภทเอกสาร')
ON CONFLICT (code) DO NOTHING;

-- master does NOT auto-receive permissions added after the initial seed, so grant
-- explicitly. Managing the list is an owner/master job; employees only read.
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM roles r CROSS JOIN permissions p
WHERE r.name IN ('master', 'owner')
  AND p.code IN ('document_types.read', 'document_types.create', 'document_types.update', 'document_types.delete')
ON CONFLICT (role_id, permission_id) DO NOTHING;

INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM roles r CROSS JOIN permissions p
WHERE r.name = 'employee' AND p.code = 'document_types.read'
ON CONFLICT (role_id, permission_id) DO NOTHING;
