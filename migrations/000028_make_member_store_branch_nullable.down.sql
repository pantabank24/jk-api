ALTER TABLE members ALTER COLUMN store_id SET NOT NULL;
ALTER TABLE members ALTER COLUMN branch_id SET NOT NULL;

ALTER TABLE credit_transactions ALTER COLUMN store_id SET NOT NULL;
ALTER TABLE credit_transactions ALTER COLUMN branch_id SET NOT NULL;
