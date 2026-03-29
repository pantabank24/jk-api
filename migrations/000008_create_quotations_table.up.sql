CREATE TABLE IF NOT EXISTS quotations (
    id SERIAL PRIMARY KEY,
    store_id INTEGER NOT NULL REFERENCES stores(id) ON DELETE CASCADE,
    branch_id INTEGER NOT NULL REFERENCES branches(id) ON DELETE CASCADE,
    member_id INTEGER REFERENCES members(id) ON DELETE SET NULL,
    created_by INTEGER REFERENCES users(id) ON DELETE SET NULL,
    code VARCHAR(20) NOT NULL UNIQUE,
    status INTEGER DEFAULT 0,
    note TEXT DEFAULT '',
    total_amount DECIMAL(12,2) DEFAULT 0,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    deleted_at TIMESTAMP WITH TIME ZONE
);

CREATE INDEX idx_quotations_store_id ON quotations(store_id);
CREATE INDEX idx_quotations_branch_id ON quotations(branch_id);
CREATE INDEX idx_quotations_member_id ON quotations(member_id);
CREATE INDEX idx_quotations_created_by ON quotations(created_by);
CREATE INDEX idx_quotations_code ON quotations(code);
CREATE INDEX idx_quotations_status ON quotations(status);
CREATE INDEX idx_quotations_deleted_at ON quotations(deleted_at);
