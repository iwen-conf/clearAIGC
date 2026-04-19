CREATE TABLE IF NOT EXISTS quality_reports (
    id UUID PRIMARY KEY,
    round_id UUID NOT NULL REFERENCES rounds(id) ON DELETE CASCADE,
    chunk_id TEXT NOT NULL,
    check_type TEXT NOT NULL,
    passed BOOLEAN NOT NULL,
    reason TEXT NOT NULL DEFAULT '',
    recovered BOOLEAN NOT NULL DEFAULT FALSE,
    recovery_method TEXT NOT NULL DEFAULT '',
    recovery_steps INT NOT NULL DEFAULT 0,
    token_cost INT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_quality_reports_round_id ON quality_reports (round_id);
CREATE INDEX IF NOT EXISTS idx_quality_reports_chunk_id ON quality_reports (chunk_id);
