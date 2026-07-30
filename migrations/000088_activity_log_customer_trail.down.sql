DROP INDEX IF EXISTS idx_activity_logs_target_user;
ALTER TABLE activity_logs DROP COLUMN IF EXISTS detail;
ALTER TABLE activity_logs DROP COLUMN IF EXISTS ref_code;
ALTER TABLE activity_logs DROP COLUMN IF EXISTS target_user_id;
