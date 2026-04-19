CREATE TABLE IF NOT EXISTS agent_settings (
    name         TEXT PRIMARY KEY CHECK (name IN ('coordinator', 'lexical_mutator', 'syntax_rebuilder')),
    display_name TEXT NOT NULL,
    protocol     TEXT NOT NULL CHECK (protocol IN ('responses', 'chat')),
    base_url     TEXT NOT NULL DEFAULT '',
    api_key      TEXT NOT NULL DEFAULT '',
    model        TEXT NOT NULL DEFAULT '',
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);
