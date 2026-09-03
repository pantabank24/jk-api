-- "เรียงตามครั้งล่าสุดที่ปรับสถานะ" ต้องมีเวลาของการเปลี่ยนสถานะเก็บไว้ ซึ่งเดิมไม่มี:
-- quotations มีแค่ created_at (ตอนเปิดบิล) กับ updated_at (การเขียนอะไรก็ได้ ไม่ใช่
-- เฉพาะการเปลี่ยนสถานะ) แท็บ สำเร็จ/เคลียร์แล้ว จึงต้องตกไปเรียงด้วย id คือ "ลำดับที่
-- บิลถูกเปิด" ซึ่งไม่เกี่ยวกับตอนที่มันถูกปิดเลย
ALTER TABLE quotations ADD COLUMN IF NOT EXISTS status_changed_at TIMESTAMPTZ;

-- ลิสต์บิลเรียงด้วยคอลัมน์นี้ทุกแท็บ (ยกเว้น รอออกบิล) และกรอง status อยู่แล้ว
CREATE INDEX IF NOT EXISTS idx_quotations_status_changed_at
    ON quotations (status, status_changed_at DESC);

-- Backfill ของเดิม แหล่งที่มาต่างกันตามสถานะ — ลำดับใน COALESCE สำคัญ: ใบที่ "สำเร็จ"
-- ก็มี issued_quotation เหมือนกัน ถ้าไล่รวมกันหมดมันจะไปหยิบ "ตอนออกใบ" มาแทน
-- "ตอนอนุมัติ" ซึ่งคนละเหตุการณ์กัน
--   14 เคลียร์แล้ว → bill_balances.settled_at เขียนใน transaction เดียวกับตอนเปลี่ยน
--                    สถานะ (ดู ClearBills) จึงเป็นเวลาที่เคลียร์จริง
--   11 รอตรวจบิล   → created_at ของใบเสนอราคาที่ออก คือตอนที่บิลเข้าแท็บนี้พอดี
--   ที่เหลือ        → updated_at ค่าที่ใกล้ที่สุดเท่าที่มี (ถ้าบิลถูกแก้ต่อหลังเปลี่ยน
--                    สถานะ ค่านี้จะเลยไปบ้าง — ยอมรับได้สำหรับข้อมูลเก่า ส่วนบิลที่
--                    เปลี่ยนสถานะหลังจากนี้แม่นเพราะโค้ดเขียนค่าเอง)
UPDATE quotations q
   SET status_changed_at = COALESCE(
       CASE
         WHEN q.status = 14 THEN
           (SELECT MAX(bb.settled_at) FROM bill_balances bb
             WHERE bb.settled_at IS NOT NULL
               AND bb.quotation_id IN (q.id, q.issued_quotation_id))
         WHEN q.status = 11 THEN
           (SELECT iq.created_at FROM quotations iq
             WHERE iq.id = q.issued_quotation_id AND iq.deleted_at IS NULL)
       END,
       q.updated_at,
       q.created_at)
 WHERE q.status_changed_at IS NULL;
