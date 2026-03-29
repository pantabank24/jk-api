CREATE TABLE IF NOT EXISTS activity_logs (
    id          BIGSERIAL PRIMARY KEY,
    user_id     BIGINT REFERENCES users(id) ON DELETE SET NULL,
    method      VARCHAR(10)  NOT NULL DEFAULT '',
    path        VARCHAR(500) NOT NULL DEFAULT '',
    status_code INT          NOT NULL DEFAULT 0,
    ip          VARCHAR(45)  NOT NULL DEFAULT '',
    user_agent  TEXT         NOT NULL DEFAULT '',
    duration_ms BIGINT       NOT NULL DEFAULT 0,
    created_at  TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_activity_logs_user_id    ON activity_logs(user_id);
CREATE INDEX idx_activity_logs_created_at ON activity_logs(created_at DESC);
CREATE INDEX idx_activity_logs_method     ON activity_logs(method);
CREATE INDEX idx_activity_logs_status     ON activity_logs(status_code);
