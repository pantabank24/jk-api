-- บันทึกใบเสร็จแอดมิน — a plain data-entry module for receipts issued OUTSIDE this
-- system (the paper form is typed in after the fact). It deliberately touches
-- nothing else: no bills, no credit, no stock. The receipt number is whatever the
-- paper says, so it is NOT generated here and NOT unique.

-- One row (id = 1) holding everything on the form that is the same on every
-- receipt. The admin edits it from a modal on the list page.
CREATE TABLE IF NOT EXISTS receipt_settings (
    id               SMALLINT     PRIMARY KEY DEFAULT 1,
    logo_url         VARCHAR(500) NOT NULL DEFAULT '',
    company_name     VARCHAR(255) NOT NULL DEFAULT '',
    -- Free text, one address line per newline: the printed header must come out
    -- exactly as the shop writes it, so it is not split into columns.
    company_address  TEXT         NOT NULL DEFAULT '',
    company_tax_id   VARCHAR(50)  NOT NULL DEFAULT '',
    company_phone    VARCHAR(50)  NOT NULL DEFAULT '',
    doc_title        VARCHAR(255) NOT NULL DEFAULT 'ใบกำกับภาษี/ใบเสร็จรับเงิน',
    seller_name      VARCHAR(255) NOT NULL DEFAULT '',
    account_name     VARCHAR(255) NOT NULL DEFAULT '',
    bank_name        VARCHAR(255) NOT NULL DEFAULT '',
    updated_at       TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    -- Pins the table to a single row; the API upserts id = 1.
    CONSTRAINT receipt_settings_singleton CHECK (id = 1)
);

INSERT INTO receipt_settings (
    id, company_name, company_address, company_tax_id, company_phone,
    seller_name, account_name, bank_name
) VALUES (
    1,
    'บริษัท เจเค โกลด์ แอนด์ ไดมอนด์ จำกัด',
    E'3/10 Emblazon เสรีไทย 43 แขวงคลองกุ่ม\nเขตบึงกุ่ม กรุงเทพมหานคร 10240',
    '0805568002668',
    '063-932-5566',
    'วีระชัย  ชัยนุมาศ',
    'บริษัท เจเค โกลด์ แอนด์ ไดมอนด์ จำกัด',
    'กสิกรไทย'
) ON CONFLICT (id) DO NOTHING;

CREATE TABLE IF NOT EXISTS admin_receipts (
    id               SERIAL       PRIMARY KEY,
    -- เลขที่ from the paper. Typed by hand, so duplicates are possible and allowed.
    code             VARCHAR(100) NOT NULL DEFAULT '',
    issued_date      DATE         NOT NULL,
    reference        VARCHAR(255) NOT NULL DEFAULT '',
    customer_name    VARCHAR(255) NOT NULL DEFAULT '',
    customer_address TEXT         NOT NULL DEFAULT '',
    customer_tax_id  VARCHAR(50)  NOT NULL DEFAULT '',
    -- รับชำระโดย is two independent ticks on the form, not a single choice.
    pay_cash         BOOLEAN      NOT NULL DEFAULT FALSE,
    pay_cheque       BOOLEAN      NOT NULL DEFAULT FALSE,
    bank_name        VARCHAR(255) NOT NULL DEFAULT '',
    cheque_no        VARCHAR(100) NOT NULL DEFAULT '',
    cheque_date      DATE         NULL,
    -- Sum of the item lines (computed on save).
    total_amount     DECIMAL(16,2) NOT NULL DEFAULT 0,
    -- What the payment block says was actually paid. Defaults to total_amount but
    -- stays separate: the paper form these are copied from can disagree with its
    -- own total, and the entry must reproduce the paper, not correct it.
    paid_amount      DECIMAL(16,2) NOT NULL DEFAULT 0,
    created_by       INT          NULL REFERENCES users(id) ON DELETE SET NULL,
    created_at       TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at       TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    deleted_at       TIMESTAMPTZ  NULL
);

CREATE INDEX IF NOT EXISTS idx_admin_receipts_deleted_at ON admin_receipts (deleted_at);
CREATE INDEX IF NOT EXISTS idx_admin_receipts_issued_date ON admin_receipts (issued_date DESC);
CREATE INDEX IF NOT EXISTS idx_admin_receipts_code ON admin_receipts (code);

CREATE TABLE IF NOT EXISTS admin_receipt_items (
    id          SERIAL        PRIMARY KEY,
    receipt_id  INT           NOT NULL REFERENCES admin_receipts(id) ON DELETE CASCADE,
    sort_order  INT           NOT NULL DEFAULT 0,
    description VARCHAR(255)  NOT NULL DEFAULT '',
    quantity    DECIMAL(16,4) NOT NULL DEFAULT 0,
    -- หน่วย printed next to จำนวน ("1000 กรัม"). Free text — the form takes grams,
    -- but nothing stops a receipt from being written in ชิ้น or บาท.
    unit        VARCHAR(50)   NOT NULL DEFAULT '',
    -- 4 decimals, printed at 2: the paper's unit prices carry more precision than
    -- the column they are printed in.
    unit_price  DECIMAL(16,4) NOT NULL DEFAULT 0,
    -- The รวม column as written on the paper — typed, never derived. A real
    -- receipt's line total does not always equal quantity x unit_price, and this
    -- table records the receipt rather than recomputing it. Only the grand total
    -- on admin_receipts is added up.
    amount      DECIMAL(16,2) NOT NULL DEFAULT 0,
    created_at  TIMESTAMPTZ   NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ   NOT NULL DEFAULT NOW(),
    deleted_at  TIMESTAMPTZ   NULL
);

CREATE INDEX IF NOT EXISTS idx_admin_receipt_items_receipt_id ON admin_receipt_items (receipt_id);
CREATE INDEX IF NOT EXISTS idx_admin_receipt_items_deleted_at ON admin_receipt_items (deleted_at);

INSERT INTO permissions (code, name, group_name, description) VALUES
    ('receipts.read',   'ดูใบเสร็จแอดมิน',    'receipts', 'ดูรายการใบเสร็จที่บันทึกไว้'),
    ('receipts.create', 'บันทึกใบเสร็จ',      'receipts', 'บันทึกใบเสร็จใหม่'),
    ('receipts.update', 'แก้ไขใบเสร็จ',       'receipts', 'แก้ไขใบเสร็จและตั้งค่าเริ่มต้น'),
    ('receipts.delete', 'ลบใบเสร็จ',          'receipts', 'ลบใบเสร็จที่บันทึกไว้')
ON CONFLICT (code) DO NOTHING;

-- Granted to NO role on purpose. master passes every permission check without a
-- row (see middleware.RequirePermission), so this ships as master-only — which is
-- what was asked. The permissions still exist, so จัดการสิทธิ์ can hand them to
-- someone else later without a migration.
