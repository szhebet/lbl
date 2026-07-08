-- Migration 2.5: Add refresh_tokens table for refresh token support
-- Allows mobile clients to refresh session tokens without re-entering password

CREATE TABLE IF NOT EXISTS refresh_tokens (
    id                 SERIAL PRIMARY KEY,
    user_id            INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash         VARCHAR(64) NOT NULL UNIQUE,  -- SHA-256 hex of the raw token
    device_name        VARCHAR(255) DEFAULT '',
    device_fingerprint VARCHAR(255) DEFAULT '',
    created_at         TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_refresh_tokens_hash ON refresh_tokens(token_hash);
CREATE INDEX IF NOT EXISTS idx_refresh_tokens_user ON refresh_tokens(user_id);
