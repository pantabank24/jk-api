DELETE FROM role_permissions WHERE permission_id IN (
    SELECT id FROM permissions WHERE code IN
        ('receipts.read', 'receipts.create', 'receipts.update', 'receipts.delete')
);
DELETE FROM permissions WHERE code IN
    ('receipts.read', 'receipts.create', 'receipts.update', 'receipts.delete');

DROP TABLE IF EXISTS admin_receipt_items;
DROP TABLE IF EXISTS admin_receipts;
DROP TABLE IF EXISTS receipt_settings;
