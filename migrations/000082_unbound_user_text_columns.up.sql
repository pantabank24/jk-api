-- The customer form's free-text fields were capped by their varchar lengths, so a
-- long store name / account name / phone-with-notes would be rejected or truncated.
-- Switch them to unbounded text. (address was already text; password stays varchar
-- since it only ever holds a fixed-length bcrypt hash, and avatar/role are not
-- user-entered form fields.)
ALTER TABLE users ALTER COLUMN name              TYPE text;
ALTER TABLE users ALTER COLUMN email             TYPE text;
ALTER TABLE users ALTER COLUMN phone             TYPE text;
ALTER TABLE users ALTER COLUMN store_name        TYPE text;
ALTER TABLE users ALTER COLUMN tax_id            TYPE text;
ALTER TABLE users ALTER COLUMN bank_account_no   TYPE text;
ALTER TABLE users ALTER COLUMN bank_account_name TYPE text;
