-- Back to the per-customer pile of migration 91. The latch starts empty again:
-- a shop-wide lot count can't be split back across the customers that made it up.

DROP TABLE IF EXISTS line_pending_sell_alerts;

CREATE TABLE line_pending_sell_alerts (
    id           BIGSERIAL PRIMARY KEY,
    user_id      BIGINT       NOT NULL,
    metal        VARCHAR(20)  NOT NULL,
    alerted_lots INT          NOT NULL DEFAULT 0,
    created_at   TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at   TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT line_pending_sell_alerts_user_metal_key UNIQUE (user_id, metal)
);

DELETE FROM system_configs WHERE key = 'line_pending_sell_name';
