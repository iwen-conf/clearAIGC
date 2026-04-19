CREATE TABLE IF NOT EXISTS rounds (
    id UUID PRIMARY KEY,
    session_id UUID NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,
    round_number INT NOT NULL CHECK (round_number BETWEEN 1 AND 3),
    prompt TEXT NOT NULL,
    prompt_profile TEXT NOT NULL CHECK (prompt_profile IN ('cn', 'en')),
    input_path TEXT NOT NULL,
    output_path TEXT NOT NULL DEFAULT '',
    score_total INT NULL CHECK (score_total BETWEEN 0 AND 70),
    chunk_limit INT NOT NULL,
    input_segment_count INT NOT NULL DEFAULT 0,
    output_segment_count INT NOT NULL DEFAULT 0,
    checkpoint_id TEXT NOT NULL,
    provider_used TEXT NOT NULL DEFAULT '',
    total_tokens BIGINT NOT NULL DEFAULT 0,
    status TEXT NOT NULL CHECK (status IN ('pending', 'processing', 'paused', 'completed', 'failed')),
    started_at TIMESTAMPTZ NULL,
    completed_at TIMESTAMPTZ NULL,
    created_at TIMESTAMPTZ NOT NULL,
    recovery_justification TEXT NOT NULL DEFAULT '',
    UNIQUE(session_id, round_number)
);

CREATE INDEX IF NOT EXISTS idx_rounds_session_id ON rounds (session_id);
CREATE INDEX IF NOT EXISTS idx_rounds_status ON rounds (status);
CREATE INDEX IF NOT EXISTS idx_rounds_checkpoint_id ON rounds (checkpoint_id);
