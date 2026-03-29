-- Add reject_reason column to quotations
ALTER TABLE quotations ADD COLUMN IF NOT EXISTS reject_reason TEXT NOT NULL DEFAULT '';

-- Ensure owner role has quotations.update permission (for existing databases)
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM roles r CROSS JOIN permissions p
WHERE r.name = 'owner' AND p.code = 'quotations.update'
ON CONFLICT (role_id, permission_id) DO NOTHING;
