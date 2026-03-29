-- Link members to user accounts (employee = member)
ALTER TABLE members ADD COLUMN IF NOT EXISTS user_id BIGINT REFERENCES users(id) ON DELETE SET NULL;
CREATE UNIQUE INDEX IF NOT EXISTS idx_members_user_id ON members(user_id) WHERE user_id IS NOT NULL;

-- Allow master users to create quotations without a store/branch
ALTER TABLE quotations ALTER COLUMN store_id  DROP NOT NULL;
ALTER TABLE quotations ALTER COLUMN branch_id DROP NOT NULL;
