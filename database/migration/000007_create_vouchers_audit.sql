CREATE TABLE IF NOT EXISTS vouchers (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    code VARCHAR(64) NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    discount_type VARCHAR(32) NOT NULL,
    discount_value NUMERIC(14, 2) NOT NULL CHECK (discount_value >= 0),
    min_order_amount NUMERIC(14, 2) NOT NULL DEFAULT 0,
    quota INTEGER NOT NULL DEFAULT 0,
    used_count INTEGER NOT NULL DEFAULT 0,
    starts_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at TIMESTAMPTZ NOT NULL,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT vouchers_discount_type_allowed CHECK (discount_type IN ('percent', 'fixed')),
    CONSTRAINT vouchers_quota_nonneg CHECK (quota >= 0),
    CONSTRAINT vouchers_used_nonneg CHECK (used_count >= 0)
);

CREATE UNIQUE INDEX IF NOT EXISTS vouchers_code_unique ON vouchers (lower(code));

CREATE TABLE IF NOT EXISTS audit_logs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    actor_user_id UUID,
    actor_email VARCHAR(320) NOT NULL DEFAULT '',
    action VARCHAR(120) NOT NULL,
    resource_type VARCHAR(80) NOT NULL,
    resource_id VARCHAR(80) NOT NULL DEFAULT '',
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS audit_logs_created_at_idx ON audit_logs (created_at DESC);
CREATE INDEX IF NOT EXISTS audit_logs_resource_idx ON audit_logs (resource_type, resource_id);
