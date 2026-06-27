INSERT INTO members (user_id, store_id, branch_id, code, fname, lname, phone, credits, status, created_at, updated_at)
SELECT
    u.id                                                                AS user_id,
    u.store_id,
    u.branch_id,
    'USR' || LPAD(u.id::text, 4, '0')                                 AS code,
    SPLIT_PART(u.name, ' ', 1)                                         AS fname,
    COALESCE(NULLIF(SPLIT_PART(u.name, ' ', 2), ''), SPLIT_PART(u.name, ' ', 1)) AS lname,
    u.phone,
    0                                                                   AS credits,
    0                                                                   AS status,
    NOW()                                                               AS created_at,
    NOW()                                                               AS updated_at
FROM users u
WHERE u.deleted_at IS NULL
  AND NOT EXISTS (
      SELECT 1 FROM members m WHERE m.user_id = u.id AND m.deleted_at IS NULL
  );
