DROP INDEX IF EXISTS idx_users_bank_id;
ALTER TABLE users DROP COLUMN IF EXISTS bank_account_name;
ALTER TABLE users DROP COLUMN IF EXISTS bank_account_no;
ALTER TABLE users DROP COLUMN IF EXISTS bank_id;
