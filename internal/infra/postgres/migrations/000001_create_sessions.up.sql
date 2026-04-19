CREATE TABLE IF NOT EXISTS sessions (
    id UUID PRIMARY KEY,
    document_id UUID NOT NULL,
    document_name TEXT NOT NULL,
    doc_id TEXT NOT NULL UNIQUE,
    origin_path TEXT NOT NULL,
    file_format TEXT NOT NULL CHECK (file_format IN ('txt', 'docx')),
    file_size_bytes BIGINT NOT NULL DEFAULT 0,
    prompt_profile TEXT NOT NULL CHECK (prompt_profile IN ('cn', 'en')),
    status TEXT NOT NULL CHECK (status IN ('pending', 'processing', 'paused', 'completed', 'failed')),
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_sessions_status_created_at ON sessions (status, created_at DESC);
