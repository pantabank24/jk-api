ALTER TABLE quotations ALTER COLUMN store_id  SET NOT NULL;
ALTER TABLE quotations ALTER COLUMN branch_id SET NOT NULL;

DROP INDEX IF EXISTS idx_members_user_id;
ALTER TABLE members DROP COLUMN IF EXISTS user_id;
