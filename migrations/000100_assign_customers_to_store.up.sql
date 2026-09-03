-- ลูกค้าเป็นข้อมูลระดับ "ร้าน": ทุกสาขาในร้านเดียวกันเห็นลูกค้าชุดเดียวกัน แต่ร้านอื่น
-- ต้องไม่เห็น. เดิม users.store_id ของลูกค้าเป็น NULL เสมอ (CreateCustomer ไม่เคย
-- เขียนลงไป) การกรองตามร้านจึงได้ 0 แถวเสมอ — ต้องผูกลูกค้าเดิมเข้าร้านก่อน
-- ตัวกรองถึงจะทำงานได้จริง.
--
-- ทำเฉพาะตอนที่ระบบมี "ร้านเดียว" เท่านั้น: ถ้ามีหลายร้านแล้ว ไม่มีทางรู้ว่าลูกค้า
-- คนไหนเป็นของร้านไหน การเดาคือการย้ายลูกค้าไปให้ร้านที่ไม่ใช่เจ้าของ. ในกรณีนั้น
-- ลูกค้าที่ store_id ยังเป็น NULL จะเห็นได้เฉพาะ master จนกว่าจะถูกกำหนดร้านด้วยมือ.
DO $$
DECLARE
    store_count INT;
    only_store  INT;
    moved       INT;
BEGIN
    SELECT count(*), min(id) INTO store_count, only_store
      FROM stores WHERE deleted_at IS NULL;

    IF store_count = 1 THEN
        UPDATE users
           SET store_id = only_store, updated_at = NOW()
         WHERE store_id IS NULL
           AND deleted_at IS NULL
           AND role_id = (SELECT id FROM roles WHERE name = 'customer');
        GET DIAGNOSTICS moved = ROW_COUNT;
        RAISE NOTICE 'ผูกลูกค้า % คน เข้าร้าน id=%', moved, only_store;

        -- members สะท้อน store/branch ของ user (ดู migration 000030) — บิลของลูกค้า
        -- ยังไม่ผูกสาขาโดยตั้งใจ แต่ร้านของ member ควรตรงกับ user ที่มันอ้างถึง
        UPDATE members m
           SET store_id = only_store, updated_at = NOW()
          FROM users u
         WHERE m.user_id = u.id
           AND m.store_id IS NULL
           AND m.deleted_at IS NULL
           AND u.role_id = (SELECT id FROM roles WHERE name = 'customer');
    ELSE
        RAISE NOTICE 'พบ % ร้าน — ข้ามการผูกลูกค้าเข้าร้าน ต้องกำหนดเองทีละราย', store_count;
    END IF;
END $$;
