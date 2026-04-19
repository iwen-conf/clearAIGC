CREATE TABLE IF NOT EXISTS audit_log (
    id BIGSERIAL PRIMARY KEY,
    session_id UUID NULL REFERENCES sessions(id) ON DELETE SET NULL,
    action TEXT NOT NULL,
    actor TEXT NOT NULL DEFAULT 'system',
    detail JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_audit_log_session_id ON audit_log (session_id);
