DELETE FROM role_permissions WHERE permission_id IN (
    SELECT id FROM permissions WHERE code IN (
        'document_types.read', 'document_types.create', 'document_types.update', 'document_types.delete'
    )
);
DELETE FROM permissions WHERE code IN (
    'document_types.read', 'document_types.create', 'document_types.update', 'document_types.delete'
);

DROP INDEX IF EXISTS idx_customer_documents_document_type_id;
ALTER TABLE customer_documents DROP COLUMN IF EXISTS document_type_id;

DROP TABLE IF EXISTS document_types;
