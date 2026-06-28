-- Revert quotations.status default back to 0 (pending).
ALTER TABLE quotations ALTER COLUMN status SET DEFAULT 0;
