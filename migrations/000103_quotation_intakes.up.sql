-- ใบเปิดงาน (quotation intake) — ขั้นตอนที่หน้าร้านทำ "ก่อน" หลอม แล้วค่อยกลับมา
-- ออกใบเสนอราคาทีหลัง. หน้างานจริงต้องถ่ายรูปก่อนหลอม + เก็บบัตรประชาชน + ชื่อ/เบอร์
-- ตั้งแต่ตอนรับของ ซึ่งเป็นคนละจังหวะเวลากับตอนที่รู้ราคาและออกใบเสนอราคาได้.
--
-- ใบเปิดงานจงใจไม่ยุ่งกับอะไรเลย: ไม่มีรายการทอง ไม่มียอดเงิน ไม่ตัดเครดิต ไม่แตะบิล.
-- มันเป็นแค่ที่พักข้อมูล/รูป จนกว่าจะถูก "ใช้" โดยใบเสนอราคาใบหนึ่ง (quotation_id)
-- ซึ่งตอนนั้นรูปจะถูกก็อปไปติดกับใบเสนอราคาแทน. ทางเดิม (ออกใบเสนอราคาตรง ๆ โดยไม่มี
-- ใบเปิดงาน) ยังทำงานเหมือนเดิมทุกอย่าง — ใบเปิดงานเป็นทางเลือกเสริมเท่านั้น.
CREATE TABLE IF NOT EXISTS quotation_intakes (
    id             SERIAL       PRIMARY KEY,
    store_id       INT          NULL REFERENCES stores(id)   ON DELETE SET NULL,
    branch_id      INT          NULL REFERENCES branches(id) ON DELETE SET NULL,
    created_by     INT          NULL REFERENCES users(id)    ON DELETE SET NULL,
    -- ลูกค้าที่ลงทะเบียนไว้ (ไม่บังคับ) — เลือกได้เพื่อดึงชื่อ/เบอร์มาเติมให้ แต่ลูกค้า
    -- เดินเข้าที่ไม่เคยลงทะเบียนก็เปิดงานได้โดยพิมพ์ชื่อ/เบอร์เอง.
    customer_id    INT          NULL REFERENCES users(id)    ON DELETE SET NULL,
    customer_name  VARCHAR(255) NOT NULL DEFAULT '',
    customer_phone VARCHAR(30)  NOT NULL DEFAULT '',
    note           TEXT         NOT NULL DEFAULT '',
    -- 0 = รอออกใบเสนอราคา, 1 = ออกใบเสนอราคาแล้ว, 2 = ยกเลิก
    status         SMALLINT     NOT NULL DEFAULT 0,
    -- ใบเสนอราคาที่ใช้ใบเปิดงานนี้ไป (ตั้งพร้อม status = 1). ON DELETE SET NULL:
    -- ถ้าใบเสนอราคาถูกลบ ใบเปิดงานยังอยู่เป็นหลักฐานว่าเคยรับของเข้ามา.
    quotation_id   INT          NULL REFERENCES quotations(id) ON DELETE SET NULL,
    used_at        TIMESTAMPTZ  NULL,
    created_at     TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at     TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    deleted_at     TIMESTAMPTZ  NULL
);

CREATE INDEX IF NOT EXISTS idx_quotation_intakes_deleted_at ON quotation_intakes (deleted_at);
-- หน้ารายการเปิดมาที่แท็บ "รอออกใบเสนอราคา" เสมอ และกรองด้วยร้าน/สาขาของคนที่เปิดดู
CREATE INDEX IF NOT EXISTS idx_quotation_intakes_status ON quotation_intakes (status, id DESC);
CREATE INDEX IF NOT EXISTS idx_quotation_intakes_store  ON quotation_intakes (store_id, branch_id);
CREATE INDEX IF NOT EXISTS idx_quotation_intakes_created_by ON quotation_intakes (created_by);

-- รูปของใบเปิดงาน. type เดียวกับ quotation_images จะได้ก็อปข้ามไปตรง ๆ ตอนออกใบ:
-- before_melt (รูปก่อนหลอม) | id_card (รูปบัตรประชาชน).
CREATE TABLE IF NOT EXISTS quotation_intake_images (
    id         SERIAL       PRIMARY KEY,
    intake_id  INT          NOT NULL REFERENCES quotation_intakes(id) ON DELETE CASCADE,
    image_url  VARCHAR(500) NOT NULL,
    type       VARCHAR(50)  NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ  NULL
);

CREATE INDEX IF NOT EXISTS idx_quotation_intake_images_intake ON quotation_intake_images (intake_id);
CREATE INDEX IF NOT EXISTS idx_quotation_intake_images_deleted_at ON quotation_intake_images (deleted_at);

-- ไม่มี permission ใหม่โดยตั้งใจ: ใบเปิดงานคือครึ่งแรกของการออกใบเสนอราคา ใครที่ออก
-- ใบเสนอราคาได้อยู่แล้ว (quotations.create / quotations.read) ก็ควรเปิดงานได้เลย
-- โดยไม่ต้องไปตั้งสิทธิ์เพิ่มในหน้าจัดการสิทธิ์.
