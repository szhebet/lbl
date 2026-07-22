-- Migration 4.1: Add uploaded_by field to editions
ALTER TABLE editions ADD COLUMN IF NOT EXISTS uploaded_by INTEGER REFERENCES users(id) ON DELETE SET NULL;

-- Add index for faster lookups
CREATE INDEX IF NOT EXISTS idx_editions_uploaded_by ON editions(uploaded_by);

-- Update the book_details view to include uploaded_by and uploader username
CREATE OR REPLACE VIEW book_details AS
SELECT
    w.id                AS work_id,
    w.original_title,
    w.original_language,
    w.first_published,
    w.work_type,
    e.id                AS edition_id,
    e.title             AS edition_title,
    e.language          AS edition_language,
    e.isbn,
    e.publisher,
    e.year,
    e.pages,
    e.series,
    e.series_number,
    e.quality,
    e.on_shelf,
    e.shelf_order,
    -- Авторы
    (SELECT STRING_AGG(p.last_name || ' ' || p.first_name, ', ')
     FROM work_contributors wc
     JOIN persons p ON p.id = wc.person_id
     WHERE wc.work_id = w.id AND wc.role = 'author') AS authors,
    -- Переводчики
    (SELECT STRING_AGG(p.last_name || ' ' || p.first_name, ', ')
     FROM work_contributors wc
     JOIN persons p ON p.id = wc.person_id
     WHERE wc.work_id = w.id AND wc.role = 'translator') AS translators,
    -- Жанры
    (SELECT STRING_AGG(g.name, ', ')
     FROM work_genres wg
     JOIN genres g ON g.id = wg.genre_id
     WHERE wg.work_id = w.id) AS genres,
    -- Форматы
    (SELECT STRING_AGG(f.name, ', ' ORDER BY f.name)
     FROM edition_files ef
     JOIN formats f ON f.id = ef.format_id
     WHERE ef.edition_id = e.id) AS available_formats,
    -- Количество форматов
    (SELECT COUNT(*) FROM edition_files ef WHERE ef.edition_id = e.id) AS format_count,
    -- Основной файл
    (SELECT ef.file_path
     FROM edition_files ef
     WHERE ef.edition_id = e.id AND ef.is_primary = true
     LIMIT 1) AS primary_file_path,
    -- Прогресс чтения
    rp.percentage       AS reading_progress,
    rp.rating,
    rp.finished_at,
    e.upload_date,
    e.created_at,
    e.updated_at,
    -- Загрузивший пользователь
    e.uploaded_by,
    u.username          AS uploaded_by_username
FROM works w
JOIN editions e ON e.work_id = w.id
LEFT JOIN reading_progress rp ON rp.edition_id = e.id
LEFT JOIN users u ON u.id = e.uploaded_by;
