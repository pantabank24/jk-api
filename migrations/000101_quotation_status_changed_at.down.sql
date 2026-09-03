DROP INDEX IF EXISTS idx_quotations_status_changed_at;
ALTER TABLE quotations DROP COLUMN IF EXISTS status_changed_at;
