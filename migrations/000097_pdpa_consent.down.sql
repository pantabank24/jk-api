DROP TABLE IF EXISTS user_consents;

DELETE FROM system_configs WHERE key IN ('pdpa_consent_version', 'pdpa_consent_text');
