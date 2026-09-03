-- คืนลูกค้ากลับไปเป็น "ไม่สังกัดร้าน" แบบก่อน migration 000100.
-- หมายเหตุ: ล้าง store_id ของลูกค้าทุกคน รวมถึงร้านที่ถูกกำหนดด้วยมือหลัง migration นี้
-- ด้วย — ข้อมูลนั้นกู้ไม่ได้จาก down migration.
UPDATE members m
   SET store_id = NULL, updated_at = NOW()
  FROM users u
 WHERE m.user_id = u.id
   AND m.deleted_at IS NULL
   AND u.role_id = (SELECT id FROM roles WHERE name = 'customer');

UPDATE users
   SET store_id = NULL, updated_at = NOW()
 WHERE deleted_at IS NULL
   AND role_id = (SELECT id FROM roles WHERE name = 'customer');
