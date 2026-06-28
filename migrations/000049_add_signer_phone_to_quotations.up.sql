-- Phone of the person who signs (the customer/seller) — shown on the receipt.
ALTER TABLE quotations ADD COLUMN IF NOT EXISTS signer_phone VARCHAR(30) NOT NULL DEFAULT '';
