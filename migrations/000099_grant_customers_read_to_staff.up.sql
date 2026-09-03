-- ลูกค้าเป็นข้อมูลระดับร้าน ไม่ได้แยกตามสาขา (branch_id ของลูกค้าเป็น NULL เสมอ —
-- requiresBranch บังคับสาขาเฉพาะ role employee) ทุกสาขาจึงควรเห็นรายชื่อลูกค้าชุด
-- เดียวกัน. เดิม migration 000036 ให้ customers.read เฉพาะ master ("managed by
-- master") ทำให้เจ้าของร้าน/พนักงานเปิดหน้าลูกค้าไม่ได้เลย.
--
-- ขอบเขตของสิทธิ์นี้: รายชื่อ + รายละเอียดลูกค้า + tab เอกสาร (บัตรประชาชน/เล่มบัญชี)
-- ซึ่งเป็นตัวเดียวกันทั้งหมด. การ "อนุมัติ" เอกสารยังเป็นสิทธิ์แยก
-- (customers.approve_documents, migration 000094) และการแจ้งเตือน
-- "มีเอกสารรอตรวจสอบ" ยังส่งถึง master คนเดียวตามเดิม.
--
-- ไม่แตะ customers.create / update / delete — การเพิ่ม/แก้/ลบลูกค้ายังเป็นของ master.
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM roles r
CROSS JOIN permissions p
WHERE r.name IN ('owner', 'employee')
  AND p.code = 'customers.read'
ON CONFLICT (role_id, permission_id) DO NOTHING;
