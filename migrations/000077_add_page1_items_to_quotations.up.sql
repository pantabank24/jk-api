-- page1_items stores the detailed per-item breakdown (JSON array) for the printed
-- quotation's page 1. The `quotation_items` rows are stored consolidated (one line
-- per metal) for the issued quotation, so this keeps the itemised view available on
-- reprint regardless of partial-ticking or whether delivery logs were written.
ALTER TABLE quotations ADD COLUMN IF NOT EXISTS page1_items JSONB;
