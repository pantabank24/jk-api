-- เคลียร์บิล now settles the debt/credit ledger: settled rows keep their history
-- but stop contributing to the customer's balance / average-price calculation.
ALTER TABLE bill_balances ADD COLUMN IF NOT EXISTS settled_at TIMESTAMPTZ NULL;
