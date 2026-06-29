CREATE TABLE IF NOT EXISTS bill_balances (
  id         BIGSERIAL PRIMARY KEY,
  user_id    BIGINT NOT NULL,
  store_id   BIGINT,
  quotation_id BIGINT,
  amount     NUMERIC(14,2) NOT NULL DEFAULT 0,
  description TEXT NOT NULL DEFAULT '',
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_bill_balances_user_id ON bill_balances(user_id);
