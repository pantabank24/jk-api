DELETE FROM role_permissions WHERE permission_id IN (
    SELECT id FROM permissions WHERE code = 'customers.approve_documents'
);
DELETE FROM permissions WHERE code = 'customers.approve_documents';

DROP INDEX IF EXISTS idx_customer_documents_approval;

ALTER TABLE customer_documents
    DROP COLUMN IF EXISTS approval_status,
    DROP COLUMN IF EXISTS approved_by,
    DROP COLUMN IF EXISTS approved_at,
    DROP COLUMN IF EXISTS reject_reason;

ALTER TABLE document_types DROP COLUMN IF EXISTS is_high_priority;
