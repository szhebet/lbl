-- Migration 4.5: user_parents M:N self-referential relation (parent ↔ child)
-- No restrictions on the link table: a user may be their own parent and may be
-- a child of their own children (cycles allowed).

CREATE TABLE IF NOT EXISTS user_parents (
    user_id    INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    parent_id  INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (user_id, parent_id)
);

CREATE INDEX IF NOT EXISTS idx_user_parents_parent ON user_parents(parent_id);
