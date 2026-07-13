-- Customer payout account: which bank, the account number, and the name on the
-- account (which is not always the customer's own name, so it is stored separately).
ALTER TABLE users ADD COLUMN IF NOT EXISTS bank_id INT REFERENCES banks(id) ON DELETE SET NULL;
ALTER TABLE users ADD COLUMN IF NOT EXISTS bank_account_no VARCHAR(30) NOT NULL DEFAULT '';
ALTER TABLE users ADD COLUMN IF NOT EXISTS bank_account_name VARCHAR(255) NOT NULL DEFAULT '';

CREATE INDEX IF NOT EXISTS idx_users_bank_id ON users(bank_id);
