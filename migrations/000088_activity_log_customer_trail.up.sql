-- Customer-scoped activity trail.
--
-- activity_logs.user_id records WHO performed the action. That alone cannot
-- answer "what happened to this customer's bill", because every step after the
-- customer clicks sell (ออกบิล / อนุมัติ / แก้ไข / ลบ) is performed by staff, so
-- those rows carry the staff member's user_id and never surface on the
-- customer's timeline.
--
-- target_user_id records WHOM the action was about — the customer the bill
-- belongs to. The customer detail page reads user_id = X OR target_user_id = X,
-- which yields the complete trail from login through to the closed bill.
ALTER TABLE activity_logs ADD COLUMN IF NOT EXISTS target_user_id BIGINT REFERENCES users(id) ON DELETE SET NULL;

-- The document the action touched (bill/quotation code), so a row is traceable
-- back to a specific piece of paper without parsing the description text.
ALTER TABLE activity_logs ADD COLUMN IF NOT EXISTS ref_code VARCHAR(30) NOT NULL DEFAULT '';

-- Structured snapshot of the action, captured at the moment it happened. This
-- is the point of the whole feature: a bill's items can be edited afterwards,
-- so the bill itself is not evidence of the price the customer clicked at.
-- detail is written once and never updated.
ALTER TABLE activity_logs ADD COLUMN IF NOT EXISTS detail JSONB;

CREATE INDEX IF NOT EXISTS idx_activity_logs_target_user ON activity_logs(target_user_id, created_at DESC);
