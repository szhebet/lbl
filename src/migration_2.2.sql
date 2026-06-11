-- Migration 2.2: Add user_books table for tracking user reading status
DO $$ BEGIN
    CREATE TYPE user_book_status AS ENUM ('Не заполнено', 'Прочитано', 'Читаю', 'Отложил', 'Бросил');
EXCEPTION
    WHEN duplicate_object THEN NULL;
END $$;

CREATE TABLE IF NOT EXISTS user_books (
    id            SERIAL PRIMARY KEY,
    user_id       INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    edition_id    INTEGER NOT NULL REFERENCES editions(id) ON DELETE CASCADE,
    status        user_book_status NOT NULL DEFAULT 'Не заполнено',
    review        TEXT NOT NULL DEFAULT '',
    rating        INTEGER CHECK (rating >= 1 AND rating <= 10),
    date_started  DATE,
    date_read     DATE,
    created_at    TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at    TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(user_id, edition_id)
);

CREATE INDEX IF NOT EXISTS idx_user_books_user_id ON user_books(user_id);
CREATE INDEX IF NOT EXISTS idx_user_books_edition_id ON user_books(edition_id);
CREATE INDEX IF NOT EXISTS idx_user_books_status ON user_books(status);
