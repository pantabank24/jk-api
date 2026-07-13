-- Revert to the original varchar caps. Any value longer than the cap would fail
-- this cast, so a rollback must run before over-length data is entered.
ALTER TABLE users ALTER COLUMN name              TYPE varchar(255);
ALTER TABLE users ALTER COLUMN email             TYPE varchar(255);
ALTER TABLE users ALTER COLUMN phone             TYPE varchar(20);
ALTER TABLE users ALTER COLUMN store_name        TYPE varchar(255);
ALTER TABLE users ALTER COLUMN tax_id            TYPE varchar(50);
ALTER TABLE users ALTER COLUMN bank_account_no   TYPE varchar(30);
ALTER TABLE users ALTER COLUMN bank_account_name TYPE varchar(255);
