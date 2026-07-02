-- Customer profile: address on users (company name reuses store_name), plus a
-- documents table for files uploaded per customer (images / pdf / docx / xlsx).
ALTER TABLE users ADD COLUMN IF NOT EXISTS address TEXT NOT NULL DEFAULT '';

CREATE TABLE IF NOT EXISTS customer_documents (
  id          BIGSERIAL PRIMARY KEY,
  user_id     BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  file_name   VARCHAR(255) NOT NULL DEFAULT '',
  file_path   VARCHAR(500) NOT NULL DEFAULT '',
  file_ext    VARCHAR(10)  NOT NULL DEFAULT '',
  file_size   BIGINT       NOT NULL DEFAULT 0,
  uploaded_by BIGINT,
  created_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_customer_documents_user_id ON customer_documents(user_id);
