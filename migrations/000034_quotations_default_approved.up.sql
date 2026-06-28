-- Quotations are now approved immediately on creation (no pending step), so the
-- column default should reflect that. The application always sets status
-- explicitly, so this only affects rows inserted without an explicit status.
ALTER TABLE quotations ALTER COLUMN status SET DEFAULT 1;
