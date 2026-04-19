CREATE TABLE IF NOT EXISTS manifests (
    id UUID PRIMARY KEY,
    round_id UUID NOT NULL UNIQUE REFERENCES rounds(id) ON DELETE CASCADE,
    chunk_limit INT NOT NULL,
    chunk_metric TEXT NOT NULL CHECK (chunk_metric IN ('char', 'word')),
    paragraph_count INT NOT NULL,
    chunk_count INT NOT NULL,
    paragraphs JSONB NOT NULL,
    chunks JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_manifests_round_id ON manifests (round_id);
