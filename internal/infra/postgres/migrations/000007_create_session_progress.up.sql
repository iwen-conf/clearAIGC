CREATE TABLE IF NOT EXISTS session_progress (
    session_id UUID PRIMARY KEY REFERENCES sessions(id) ON DELETE CASCADE,
    round_number INT NOT NULL DEFAULT 0,
    phase TEXT NOT NULL DEFAULT '',
    completed_chunks INT NOT NULL DEFAULT 0,
    total_chunks INT NOT NULL DEFAULT 0,
    percent DOUBLE PRECISION NOT NULL DEFAULT 0,
    chunk_id TEXT NOT NULL DEFAULT '',
    paragraph_index INT NOT NULL DEFAULT 0,
    chunk_index INT NOT NULL DEFAULT 0,
    provider_used TEXT NOT NULL DEFAULT '',
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_session_progress_updated_at ON session_progress (updated_at DESC);
