-- The pending-sell alert now counts EVERY bill still at รอออกบิล together
-- instead of one customer at a time. What the shop forwards to its counterparty
-- is a kilo of metal; it makes no difference whether one customer or ten brought
-- it in, so the pile is the shop's, not a person's.
--
-- Two consequences, both handled here:
--   1. The latch is one row per metal, so the per-customer rows are meaningless
--      and the table is rebuilt. Everyone starts re-armed: the first time the
--      shop-wide pile reaches a kilo after this migration, it announces.
--   2. The message is no longer signed with the customer's name (there isn't one
--      any more) but with the shop's own, stored below so it can be changed on
--      the settings page.

INSERT INTO system_configs (key, value, description) VALUES
  ('line_pending_sell_name', 'วีรชัย ชัยนุมาศ', 'LINE: ชื่อผู้แจ้งที่จะขึ้นต้นข้อความแจ้งขาย')
ON CONFLICT (key) DO NOTHING;

DROP TABLE IF EXISTS line_pending_sell_alerts;

CREATE TABLE line_pending_sell_alerts (
    id           BIGSERIAL PRIMARY KEY,
    metal        VARCHAR(20)  NOT NULL,
    alerted_lots INT          NOT NULL DEFAULT 0,
    created_at   TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at   TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT line_pending_sell_alerts_metal_key UNIQUE (metal)
);
