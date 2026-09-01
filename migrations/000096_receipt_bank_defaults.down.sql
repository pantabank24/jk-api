ALTER TABLE admin_receipts
    ADD COLUMN IF NOT EXISTS paid_amount DECIMAL(16,2) NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS bank_name VARCHAR(255) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS cheque_no VARCHAR(100) NOT NULL DEFAULT '';

-- The dropped payment amount always equalled the total, so it restores exactly.
UPDATE admin_receipts SET paid_amount = total_amount;

UPDATE admin_receipts r SET
    bank_name = s.bank_name,
    cheque_no = s.bank_account_no
FROM receipt_settings s WHERE s.id = 1;

ALTER TABLE admin_receipts RENAME COLUMN paid_date TO cheque_date;

ALTER TABLE receipt_settings DROP COLUMN IF EXISTS bank_account_no;
