-- Which payment method was ticked on the printed quotation (ชำระโดย):
-- '' = ยังไม่ระบุ, 'cash' = เงินสด, 'transfer' = เงินโอน/บัตร/เช็ค.
-- Stored so reopening an issued quotation shows the same tick (and the bank
-- details it fills in) instead of a blank form.
ALTER TABLE quotations ADD COLUMN IF NOT EXISTS payment_method VARCHAR(20) NOT NULL DEFAULT '';
