CREATE TABLE IF NOT EXISTS members (
    id SERIAL PRIMARY KEY,
    store_id INTEGER NOT NULL REFERENCES stores(id) ON DELETE CASCADE,
    branch_id INTEGER NOT NULL REFERENCES branches(id) ON DELETE CASCADE,
    code VARCHAR(20) NOT NULL UNIQUE,
    image VARCHAR(500) DEFAULT '',
    fname VARCHAR(255) NOT NULL,
    lname VARCHAR(255) NOT NULL,
    phone VARCHAR(20) DEFAULT '',
    credits DECIMAL(12,2) DEFAULT 0,
    status INTEGER DEFAULT 0,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    deleted_at TIMESTAMP WITH TIME ZONE
);

CREATE INDEX idx_members_store_id ON members(store_id);
CREATE INDEX idx_members_branch_id ON members(branch_id);
CREATE INDEX idx_members_code ON members(code);
CREATE INDEX idx_members_deleted_at ON members(deleted_at);
