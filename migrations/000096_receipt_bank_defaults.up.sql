-- รับชำระโดย: the bank line is the shop's own account, identical on every receipt,
-- so ธนาคาร and เลขที่ (the ACCOUNT number — not a cheque number) move to the
-- defaults. Only the date is still chosen per receipt.
ALTER TABLE receipt_settings
    ADD COLUMN IF NOT EXISTS bank_account_no VARCHAR(100) NOT NULL DEFAULT '';

-- Carry over anything already typed on a receipt so nothing is silently lost if
-- receipts were entered between 000095 and this.
UPDATE receipt_settings s SET bank_account_no = sub.cheque_no
FROM (
    SELECT cheque_no FROM admin_receipts
    WHERE cheque_no <> '' ORDER BY id DESC LIMIT 1
) sub
WHERE s.id = 1 AND s.bank_account_no = '';

UPDATE receipt_settings s SET bank_name = sub.bank_name
FROM (
    SELECT bank_name FROM admin_receipts
    WHERE bank_name <> '' ORDER BY id DESC LIMIT 1
) sub
WHERE s.id = 1 AND s.bank_name = '';

-- The payment date is no longer specifically a cheque date: it is the date on the
-- รับชำระโดย line whichever way the money came in.
ALTER TABLE admin_receipts RENAME COLUMN cheque_date TO paid_date;

-- Now held in receipt_settings.
ALTER TABLE admin_receipts DROP COLUMN IF EXISTS bank_name;
ALTER TABLE admin_receipts DROP COLUMN IF EXISTS cheque_no;

-- จำนวน on the payment line always reads the receipt's own total, so storing it
-- twice only creates a second number that can drift out of step.
ALTER TABLE admin_receipts DROP COLUMN IF EXISTS paid_amount;
