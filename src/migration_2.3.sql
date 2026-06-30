-- Migration 2.3: Add read_list table for user reading lists
CREATE TABLE IF NOT EXISTS read_list (
    id          SERIAL PRIMARY KEY,
    listname    TEXT NOT NULL DEFAULT 'default',
    bookname    TEXT NOT NULL DEFAULT '',
    author      TEXT NOT NULL DEFAULT '',
    priority    INTEGER NOT NULL DEFAULT 0,
    author_id   INTEGER REFERENCES persons(id) ON DELETE SET NULL,
    book_id     INTEGER REFERENCES editions(id) ON DELETE SET NULL,
    user_id     INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    comment     TEXT NOT NULL DEFAULT '',
    status      user_book_status NOT NULL DEFAULT 'Не заполнено',
    created_at  TIMESTAMP DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_read_list_user_id ON read_list(user_id);
CREATE INDEX IF NOT EXISTS idx_read_list_listname ON read_list(listname);
