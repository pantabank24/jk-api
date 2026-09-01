-- High-priority documents (บัตรประชาชน, เล่มบัญชี, ...) are identity papers: the
-- customer may replace one but not delete it, and every new copy has to be checked
-- by staff before it counts.
ALTER TABLE document_types
    ADD COLUMN IF NOT EXISTS is_high_priority BOOLEAN NOT NULL DEFAULT FALSE;

UPDATE document_types SET is_high_priority = TRUE WHERE code = 'id_card';

-- 'approved' is the default so ordinary documents (and every row that predates this
-- migration) need no review — only high-priority uploads are written as 'pending'.
ALTER TABLE customer_documents
    ADD COLUMN IF NOT EXISTS approval_status VARCHAR(20)  NOT NULL DEFAULT 'approved',
    ADD COLUMN IF NOT EXISTS approved_by     INT          NULL REFERENCES users(id) ON DELETE SET NULL,
    ADD COLUMN IF NOT EXISTS approved_at     TIMESTAMPTZ  NULL,
    ADD COLUMN IF NOT EXISTS reject_reason   VARCHAR(500) NOT NULL DEFAULT '';

-- Drives the "รอตรวจสอบเอกสาร" badge on the customer list, which reads every
-- customer's pending rows at once.
CREATE INDEX IF NOT EXISTS idx_customer_documents_approval
    ON customer_documents (approval_status, user_id);

-- Approving identity documents is a separate job from editing a customer's details,
-- so it gets its own permission rather than riding on customers.update.
INSERT INTO permissions (code, name, group_name, description) VALUES
    ('customers.approve_documents', 'ตรวจสอบเอกสารลูกค้า', 'customers', 'อนุมัติหรือปฏิเสธเอกสารสำคัญของลูกค้า')
ON CONFLICT (code) DO NOTHING;

-- master does NOT auto-receive permissions added after the initial seed, so grant
-- explicitly. Employees review documents too — that is the point of the notification.
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM roles r CROSS JOIN permissions p
WHERE r.name IN ('master', 'owner', 'employee') AND p.code = 'customers.approve_documents'
ON CONFLICT (role_id, permission_id) DO NOTHING;
