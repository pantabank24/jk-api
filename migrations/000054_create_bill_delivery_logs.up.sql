CREATE TABLE IF NOT EXISTS bill_delivery_logs (
  id         BIGSERIAL PRIMARY KEY,
  bill_id    BIGINT NOT NULL,
  weight     NUMERIC(10,4) NOT NULL DEFAULT 0,
  amount     NUMERIC(14,2) NOT NULL DEFAULT 0,
  note       TEXT NOT NULL DEFAULT '',
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_bill_delivery_logs_bill_id ON bill_delivery_logs(bill_id);
