CREATE TABLE IF NOT EXISTS messages (
    id           BIGSERIAL PRIMARY KEY,
    session_id   TEXT        NOT NULL,
    role         TEXT        NOT NULL,
    content      TEXT        NOT NULL,
    input_token  BIGINT      NOT NULL DEFAULT 0,
    output_token BIGINT      NOT NULL DEFAULT 0,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_messages_session_id ON messages (session_id, created_at);
