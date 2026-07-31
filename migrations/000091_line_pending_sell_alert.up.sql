-- Replace the shop-wide sell-in meter (migrations 89/90) with what the shop
-- actually asked for: alert when ONE customer's pile of ยังไม่ได้ออกบิล metal
-- reaches the threshold.
--
-- It is a state alert, not a running total. The question it answers is "does
-- this customer have a full lot sitting at รอออกบิล right now" — so nothing
-- accumulates across customers, and issuing the bill takes the weight out of the
-- picture by itself. The meter keys are dropped: there is no meter any more.

INSERT INTO system_configs (key, value, description)
SELECT 'line_pending_sell_gold_enabled',
       COALESCE((SELECT value FROM system_configs WHERE key = 'line_sell_accum_gold_enabled'), 'false'),
       'LINE: แจ้งเตือนเมื่อลูกค้ามีทองรอออกบิลถึงเกณฑ์ (true/false)'
ON CONFLICT (key) DO NOTHING;

INSERT INTO system_configs (key, value, description)
SELECT 'line_pending_sell_silver_enabled',
       COALESCE((SELECT value FROM system_configs WHERE key = 'line_sell_accum_silver_enabled'), 'false'),
       'LINE: แจ้งเตือนเมื่อลูกค้ามีเงินรอออกบิลถึงเกณฑ์ (true/false)'
ON CONFLICT (key) DO NOTHING;

INSERT INTO system_configs (key, value, description)
SELECT 'line_pending_sell_threshold_gold',
       COALESCE(NULLIF((SELECT value FROM system_configs WHERE key = 'line_sell_accum_threshold_gold'), ''), '65'),
       'LINE: น้ำหนักทองรอออกบิล (บาท) ต่อ 1 กิโลกรัมที่จะแจ้งขาย (0 = ปิด)'
ON CONFLICT (key) DO NOTHING;

INSERT INTO system_configs (key, value, description)
SELECT 'line_pending_sell_threshold_silver',
       COALESCE(NULLIF((SELECT value FROM system_configs WHERE key = 'line_sell_accum_threshold_silver'), ''), '1000'),
       'LINE: น้ำหนักเงินรอออกบิล (กรัม) ต่อ 1 กิโลกรัมที่จะแจ้งขาย (0 = ปิด)'
ON CONFLICT (key) DO NOTHING;

INSERT INTO system_configs (key, value, description)
SELECT 'line_pending_sell_purity_gold',
       COALESCE(NULLIF((SELECT value FROM system_configs WHERE key = 'line_sell_accum_purity_gold'), ''), '99.99'),
       'LINE: ความบริสุทธิ์ (%) ที่ระบุในข้อความแจ้งขายทอง'
ON CONFLICT (key) DO NOTHING;

INSERT INTO system_configs (key, value, description)
SELECT 'line_pending_sell_purity_silver',
       COALESCE(NULLIF((SELECT value FROM system_configs WHERE key = 'line_sell_accum_purity_silver'), ''), '99.9'),
       'LINE: ความบริสุทธิ์ (%) ที่ระบุในข้อความแจ้งขายเงิน'
ON CONFLICT (key) DO NOTHING;

DELETE FROM system_configs WHERE key LIKE 'line_sell_accum_%';

-- One row per (customer, metal): how many whole lots we have already announced
-- for the pile they have sitting at รอออกบิล. Alert only when the pile reaches a
-- lot we haven't announced, so a customer selling all afternoon gets one message
-- per kilo, not one per click. Falls back down when the pile shrinks (staff
-- removed a line), which re-arms it.
CREATE TABLE IF NOT EXISTS line_pending_sell_alerts (
    id           BIGSERIAL PRIMARY KEY,
    user_id      BIGINT       NOT NULL,
    metal        VARCHAR(20)  NOT NULL,
    alerted_lots INT          NOT NULL DEFAULT 0,
    created_at   TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at   TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT line_pending_sell_alerts_user_metal_key UNIQUE (user_id, metal)
);
