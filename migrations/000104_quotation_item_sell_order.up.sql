-- ตั้งแต่ฟิลจากระบบขายอัตโนมัติถูกรวมเข้าบิล "รอออกบิล" ใบเดิมของลูกค้า (เหมือนตอนลูกค้า
-- กดขายเอง) ธง auto_sell ที่ระดับบิลก็ตอบไม่ได้อีกต่อไปว่า "รายการไหน" ที่ระบบขายให้ —
-- บิลใบเดียวมีได้ทั้งรายการที่ลูกค้ากดเองและรายการที่ระบบขายให้ และ
-- quotations.sell_order_id ก็เก็บได้ค่าเดียว (ออเดอร์ล่าสุดทับของเดิม)
--
-- คอลัมน์นี้ผูกออเดอร์ไว้กับ "รายการ" ที่มันขายจริง ๆ ซึ่งอยู่ได้ถาวรและมีได้หลายออเดอร์
-- ในบิลเดียวกัน — ใช้ทั้งติด Chip ในหน้าบิล/หน้าออกใบเสนอราคา และให้ boot recovery
-- ตามหาว่าฟิลนั้นลงบิลไปแล้วหรือยัง
ALTER TABLE quotation_items
    ADD COLUMN IF NOT EXISTS sell_order_id BIGINT;

-- recovery ค้นด้วยคอลัมน์นี้ทุกครั้งที่เซิร์ฟเวอร์บูตแล้วเจอออเดอร์ค้างสถานะ filling
CREATE INDEX IF NOT EXISTS idx_quotation_items_sell_order
    ON quotation_items (sell_order_id)
    WHERE sell_order_id IS NOT NULL;

-- Backfill ของเดิม: จับคู่แบบเจาะจงด้วยน้ำหนักและราคาที่ฟิลไป ไม่ใช่เหมารายการทั้งบิล
-- เพราะบิล auto_sell เก่าอาจมีรายการที่ลูกค้ากดขายเองปนอยู่แล้ว — ก่อนหน้านี้
-- FindPendingByCreator ไม่ได้กรอง auto_sell ออก การขายด้วยมือจึงไหลเข้าบิลของระบบได้
-- รายการเก่าที่จับคู่ไม่ได้ปล่อยว่างไว้ (ไม่ติด Chip) ดีกว่าติดผิดรายการ
UPDATE quotation_items qi
   SET sell_order_id = so.id
  FROM quotations q
  JOIN sell_orders so ON so.id = q.sell_order_id
 WHERE qi.quotation_id = q.id
   AND q.is_bill = TRUE
   AND q.auto_sell = TRUE
   AND q.deleted_at IS NULL
   AND qi.deleted_at IS NULL
   AND qi.sell_order_id IS NULL
   AND so.filled_price IS NOT NULL
   AND qi.weight = so.weight
   AND qi.price = so.filled_price;
