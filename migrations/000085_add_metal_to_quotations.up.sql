-- Bills are now single-metal: a customer's sells only accumulate into their open
-- "รอออกบิล" bill when the metal matches, so gold and silver never share a bill
-- (and the list pages รายการขายทอง / รายการขายเงิน can filter server-side).
--
-- The column lives on quotations (bills are quotations with is_bill = true) and
-- is also stamped on the master's issued quotation so a reprint knows its metal.

ALTER TABLE quotations ADD COLUMN IF NOT EXISTS metal VARCHAR(20) NOT NULL DEFAULT 'gold';

-- Backfill from the items: a document is tagged with its items' metal only when
-- every item agrees. Legacy MIXED bills stay 'gold' — that is the list they have
-- always appeared in, and their silver lines still show inside the bill.
UPDATE quotations q
SET metal = sub.metal
FROM (
    SELECT quotation_id, MIN(COALESCE(metal, 'gold')) AS metal
    FROM quotation_items
    WHERE deleted_at IS NULL
    GROUP BY quotation_id
    HAVING COUNT(DISTINCT COALESCE(metal, 'gold')) = 1
) sub
WHERE q.id = sub.quotation_id
  AND sub.metal <> 'gold';

-- Serves both list pages (is_bill + metal + status) and the find-or-append lookup.
CREATE INDEX IF NOT EXISTS idx_quotations_bill_metal ON quotations (is_bill, metal, status);
