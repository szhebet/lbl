-- Migration 4.4: Suggestions table for book suggestion feature
CREATE TABLE IF NOT EXISTS suggestions (
    id SERIAL PRIMARY KEY,
    read_list_id UUID NOT NULL REFERENCES read_list(id) ON DELETE CASCADE,
    edition_id INTEGER REFERENCES editions(id) ON DELETE SET NULL,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    hidden BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- unique index for suggestions without edition (one per user per read_list)
CREATE UNIQUE INDEX IF NOT EXISTS idx_suggestions_unique_no_edition
    ON suggestions(read_list_id, user_id) WHERE edition_id IS NULL;

-- unique index for suggestions with edition (one per user per read_list per edition)
CREATE UNIQUE INDEX IF NOT EXISTS idx_suggestions_unique_with_edition
    ON suggestions(read_list_id, edition_id, user_id) WHERE edition_id IS NOT NULL;
