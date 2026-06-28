-- Links a master-created quotation back to the customer's bill (the sell request
-- it was issued for). Both rows live in the quotations table.
ALTER TABLE quotations ADD COLUMN IF NOT EXISTS bill_id BIGINT REFERENCES quotations(id) ON DELETE SET NULL;
CREATE INDEX IF NOT EXISTS idx_quotations_bill_id ON quotations(bill_id);
