-- Tracks whether the credit charged for this quotation on approval has been
-- refunded back to the creator's member profile via the credit-reset action.
ALTER TABLE quotations ADD COLUMN IF NOT EXISTS credits_refunded BOOLEAN NOT NULL DEFAULT false;
