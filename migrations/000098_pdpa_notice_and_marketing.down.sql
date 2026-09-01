DROP INDEX IF EXISTS idx_user_consents_latest;
DROP INDEX IF EXISTS idx_user_consents_ack_unique;

CREATE UNIQUE INDEX IF NOT EXISTS idx_user_consents_unique
    ON user_consents (user_id, consent_type, version);

ALTER TABLE user_consents DROP COLUMN IF EXISTS granted;

DELETE FROM system_configs WHERE key = 'pdpa_marketing_text';

UPDATE system_configs SET value = '1' WHERE key = 'pdpa_consent_version';
