-- Prices for non-gold metals (silver fetched via cron; platinum/palladium are
-- entered at quotation time and not stored here). Mirrors the gold_prices
-- pattern: insert a new row per fetch, latest row per symbol = current price.
CREATE TABLE IF NOT EXISTS metal_prices (
    id           BIGSERIAL PRIMARY KEY,
    symbol       VARCHAR(10)  NOT NULL,          -- XAG (silver); reserved: XPT, XPD
    buy          DECIMAL(12,2) DEFAULT 0,
    sell         DECIMAL(12,2) DEFAULT 0,
    spot         DECIMAL(12,2) DEFAULT 0,
    exchange     DECIMAL(12,4) DEFAULT 0,
    previous     DECIMAL(12,2) DEFAULT 0,
    change_today DECIMAL(10,2) DEFAULT 0,
    price_date   VARCHAR(100) DEFAULT '',
    price_time   VARCHAR(50)  DEFAULT '',
    round        VARCHAR(50)  DEFAULT '',
    source       VARCHAR(20)  DEFAULT 'auto',    -- auto (cron) | manual
    created_at   TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_metal_prices_symbol_id ON metal_prices(symbol, id DESC);

-- Permissions (mirrors gold_prices.*)
INSERT INTO permissions (code, name, group_name, description) VALUES
    ('metal_prices.read',   'ดูราคาโลหะอื่น',     'metal_prices', 'ดูราคาเงิน/แพลตินัม/แพลเลเดียม'),
    ('metal_prices.create', 'ดึง/บันทึกราคาโลหะ', 'metal_prices', 'ดึงราคาโลหะอื่นและบันทึก')
ON CONFLICT (code) DO NOTHING;

INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM roles r CROSS JOIN permissions p
WHERE r.name = 'master' AND p.code IN ('metal_prices.read', 'metal_prices.create')
ON CONFLICT (role_id, permission_id) DO NOTHING;
