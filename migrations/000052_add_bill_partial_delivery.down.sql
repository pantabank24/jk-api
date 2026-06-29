ALTER TABLE quotations
  DROP COLUMN IF EXISTS processed_weight,
  DROP COLUMN IF EXISTS processed_amount;
