DELETE FROM members WHERE code ~ '^USR[0-9]+$' AND user_id IS NOT NULL;
