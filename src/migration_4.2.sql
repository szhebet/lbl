CREATE TABLE IF NOT EXISTS shelf_tokens (
    id          SERIAL PRIMARY KEY,
    token       VARCHAR(64) NOT NULL UNIQUE,
    edition_id  INTEGER NOT NULL REFERENCES editions(id) ON DELETE CASCADE,
    created_at  TIMESTAMP DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_shelf_tokens_token ON shelf_tokens(token);
CREATE INDEX IF NOT EXISTS idx_shelf_tokens_edition ON shelf_tokens(edition_id);
