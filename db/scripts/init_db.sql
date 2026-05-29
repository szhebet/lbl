-- ============================================================
-- СКРИПТ СОЗДАНИЯ БАЗЫ ДАННЫХ ДОМАШНЕЙ БИБЛИОТЕКИ
-- PostgreSQL 15+
-- вызывается в приложении и создает БД, если БД не найдена
-- 
-- ============================================================

-- Создаем свежую базу данных
CREATE DATABASE __DB_NAME__ OWNER __DB_USER__ ENCODING 'UTF8';

-- Подключаемся к новой базе данных
\c __DB_NAME__;

-- ============================================================
-- РАСШИРЕНИЯ
-- ============================================================
CREATE EXTENSION IF NOT EXISTS "pg_trgm";      -- нечёткий поиск
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";    -- генерация UUID

-- ============================================================
-- ВЕРСИЯ БАЗЫ ДАННЫХ
-- ============================================================

CREATE TABLE db_version (
    version     VARCHAR(20) NOT NULL,
    updated_at  TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

INSERT INTO db_version (version) VALUES ('1.0');

-- ============================================================
-- ТАБЛИЦЫ АУТЕНТИФИКАЦИИ И ПОЛЬЗОВАТЕЛЕЙ
-- ============================================================

CREATE TABLE users (
    id            SERIAL PRIMARY KEY,
    username      VARCHAR(100) NOT NULL UNIQUE,
    password_hash VARCHAR(255) NOT NULL,
    email         VARCHAR(255) UNIQUE,
    role          VARCHAR(20) NOT NULL DEFAULT 'viewer',  -- viewer, editor, admin
    created_at    TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at    TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_users_username ON users(username);
CREATE INDEX IF NOT EXISTS idx_users_email ON users(email);

-- ============================================================
-- ТАБЛИЦЫ АУТЕНТИФИКАЦИИ И ПОЛЬЗОВАТЕЛЕЙ
-- ============================================================

-- Форматы книг
CREATE TABLE formats (
    id          SERIAL PRIMARY KEY,
    name        VARCHAR(50)  NOT NULL UNIQUE,
    extension   VARCHAR(10)  NOT NULL,
    mime_type   VARCHAR(100),
    category    VARCHAR(50)  NOT NULL,  -- ebook, document, scanned, audiobook, comics
    is_reflowable BOOLEAN DEFAULT true,
    is_editable   BOOLEAN DEFAULT false
);

INSERT INTO formats (name, extension, mime_type, category, is_reflowable, is_editable) VALUES
('FB2',   'fb2',   'application/x-fictionbook+xml',                           'ebook',    true,  false),
('FB2.ZIP','fb2.zip','application/x-zip-compressed',                          'ebook',    true,  false),
('EPUB',  'epub',  'application/epub+zip',                                    'ebook',    true,  false),
('MOBI',  'mobi',  'application/x-mobipocket-ebook',                          'ebook',    true,  false),
('AZW3',  'azw3',  'application/vnd.amazon.ebook',                            'ebook',    true,  false),
('PDF',   'pdf',   'application/pdf',                                         'document', false, false),
('DJVU',  'djvu',  'image/vnd.djvu',                                          'scanned',  false, false),
('DOC',   'doc',   'application/msword',                                      'document', true,  true),
('DOCX',  'docx',  'application/vnd.openxmlformats-officedocument.wordprocessingml.document', 'document', true, true),
('RTF',   'rtf',   'application/rtf',                                         'document', true,  true),
('TXT',   'txt',   'text/plain',                                              'ebook',    true,  true),
('HTML',  'html',  'text/html',                                               'ebook',    true,  true),
('CBZ',   'cbz',   'application/x-cbz',                                       'comics',   false, false),
('CBR',   'cbr',   'application/x-cbr',                                       'comics',   false, false);

-- Языки
CREATE TABLE languages (
    code        VARCHAR(3)  PRIMARY KEY,   -- ISO 639-2/B
    name        VARCHAR(100) NOT NULL,
    native_name VARCHAR(100)
);

-- Частично заполним самые ходовые
INSERT INTO languages (code, name, native_name) VALUES
('rus', 'Russian',  'Русский'),
('eng', 'English',  'English'),
('deu', 'German',   'Deutsch'),
('fra', 'French',   'Français'),
('spa', 'Spanish',  'Español'),
('ita', 'Italian',  'Italiano'),
('jpn', 'Japanese', '日本語'),
('chi', 'Chinese',  '中文'),
('ara', 'Arabic',   'العربية'),
('por', 'Portuguese','Português'),
('ukr', 'Ukrainian','Українська'),
('bel', 'Belarusian','Беларуская');

-- ============================================================
-- ПРОИЗВЕДЕНИЯ И АВТОРЫ
-- ============================================================

-- Произведение (абстрактная «книга как идея»)
CREATE TABLE works (
    id                SERIAL PRIMARY KEY,
    original_title    TEXT NOT NULL,
    original_language VARCHAR(3) REFERENCES languages(code),
    first_published   INTEGER,                -- год первой публикации
    work_type         VARCHAR(50) DEFAULT 'novel',  -- novel, story, poem, collection, article
    annotation        TEXT,                   -- описание произведения, общее для всех изданий
    word_count        INTEGER,                -- примерное число слов
    created_at              TIMESTAMP DEFAULT NOW(),
    updated_at              TIMESTAMP DEFAULT NOW(),
    lower_original_title    TEXT
);

CREATE INDEX idx_works_lower_title ON works USING gin (lower_original_title gin_trgm_ops);

-- Персоны (авторы, переводчики, редакторы и т.д.)
CREATE TABLE persons (
    id          SERIAL PRIMARY KEY,
    first_name  TEXT,
    middle_name TEXT,
    last_name   TEXT NOT NULL,
    pseudonym   TEXT,                         -- основной псевдоним, если есть
    birth_date  DATE,
    death_date  DATE,
    biography   TEXT,
    photo_url   TEXT,
    created_at  TIMESTAMP DEFAULT NOW(),
    lower_fio   VARCHAR(510),
    UNIQUE (first_name, last_name)
);

CREATE INDEX idx_persons_lower_fio ON persons USING gin (lower_fio gin_trgm_ops);

-- Роли персон
CREATE TYPE contributor_role AS ENUM (
    'author', 'translator', 'editor', 'illustrator', 'compiler',
    'preface_author', 'commentary_author', 'other'
);

-- Связь произведения и персон
CREATE TABLE work_contributors (
    work_id   INTEGER REFERENCES works(id) ON DELETE CASCADE,
    person_id INTEGER REFERENCES persons(id) ON DELETE CASCADE,
    role      contributor_role NOT NULL,
    PRIMARY KEY (work_id, person_id, role)
);

-- Иерархия жанров
CREATE TABLE genres (
    id          SERIAL PRIMARY KEY,
    name        TEXT NOT NULL UNIQUE,
    parent_id   INTEGER REFERENCES genres(id),
    description TEXT
);

-- Связь произведения и жанров
CREATE TABLE work_genres (
    work_id  INTEGER REFERENCES works(id) ON DELETE CASCADE,
    genre_id INTEGER REFERENCES genres(id) ON DELETE CASCADE,
    PRIMARY KEY (work_id, genre_id)
);

-- ============================================================
-- ИЗДАНИЯ И ФАЙЛЫ
-- ============================================================

-- Издание (конкретная книга в конкретном языке, переводе, оформлении)
CREATE TABLE editions (
    id            SERIAL PRIMARY KEY,
    work_id       INTEGER NOT NULL REFERENCES works(id) ON DELETE CASCADE,
    
    -- Идентификаторы
    isbn          VARCHAR(50) UNIQUE,         -- с дефисами или без
    ean           VARCHAR(13),
    udc           VARCHAR(50),                -- УДК
    bbk           VARCHAR(50),                -- ББК
    
    -- Метаданные конкретного издания
    title         TEXT NOT NULL,              -- может отличаться от original_title
    language      VARCHAR(3) REFERENCES languages(code),
    publisher     TEXT,
    year          INTEGER,
    city          TEXT,
    pages         INTEGER,
    series        TEXT,
    series_number VARCHAR(50),
    annotation    TEXT,                       -- аннотация конкретного издания
    
    -- Статус и происхождение
    source        VARCHAR(255),               -- откуда получено (libgen, flibusta, самодел)
    is_complete   BOOLEAN DEFAULT true,       -- полное издание или фрагмент
    quality       VARCHAR(20) DEFAULT 'good', -- excellent, good, poor
    on_shelf      BOOLEAN DEFAULT false,
    shelf_order   INTEGER DEFAULT 0,
    
    -- Служебное
    created_at    TIMESTAMP DEFAULT NOW(),
    updated_at    TIMESTAMP DEFAULT NOW(),
    upload_date   TIMESTAMP DEFAULT NOW(),
    cover_path    TEXT,
    lower_title   TEXT
);

CREATE INDEX idx_editions_lower_title ON editions USING gin (lower_title gin_trgm_ops);

-- Конкретные файлы изданий (одно издание может быть в нескольких форматах)
CREATE TABLE edition_files (
    id            SERIAL PRIMARY KEY,
    edition_id    INTEGER NOT NULL REFERENCES editions(id) ON DELETE CASCADE,
    format_id     INTEGER NOT NULL REFERENCES formats(id) ON DELETE RESTRICT,
    
    file_path     TEXT NOT NULL,              -- полный путь к файлу
    file_size     BIGINT,                     -- байт
    file_hash     VARCHAR(64),               -- SHA-256 для поиска побитовых дубликатов
    page_count    INTEGER,                   -- реальное число страниц в файле
    word_count    INTEGER,
    
    -- Особенности файла
    has_ocr       BOOLEAN DEFAULT false,      -- для PDF/DjVu
    has_bookmarks BOOLEAN DEFAULT false,
    has_images    BOOLEAN DEFAULT true,
    is_drm        BOOLEAN DEFAULT false,
    is_primary    BOOLEAN DEFAULT false,      -- основной файл этого издания
    
    -- История конвертации
    source_file_id INTEGER REFERENCES edition_files(id),  -- из какого файла сконвертирован
    converter      TEXT,                                  -- calibre, pandoc, libreoffice
    
    created_at    TIMESTAMP DEFAULT NOW(),
    updated_at    TIMESTAMP DEFAULT NOW(),
    
    UNIQUE(edition_id, format_id),
    UNIQUE(file_hash)
);

-- ============================================================
-- ТЕГИ И КОЛЛЕКЦИИ
-- ============================================================

-- Пользовательские теги
CREATE TABLE tags (
    id          SERIAL PRIMARY KEY,
    name        TEXT NOT NULL UNIQUE,
    color       VARCHAR(7),                  -- #RRGGBB
    description TEXT
);

-- Связь изданий с тегами
CREATE TABLE edition_tags (
    edition_id INTEGER REFERENCES editions(id) ON DELETE CASCADE,
    tag_id     INTEGER REFERENCES tags(id) ON DELETE CASCADE,
    PRIMARY KEY (edition_id, tag_id)
);

-- Коллекции/подборки
CREATE TABLE collections (
    id          SERIAL PRIMARY KEY,
    name        TEXT NOT NULL,
    description TEXT,
    is_public   BOOLEAN DEFAULT false,
    created_at  TIMESTAMP DEFAULT NOW(),
    updated_at  TIMESTAMP DEFAULT NOW()
);

-- Книги в коллекциях
CREATE TABLE collection_items (
    id            SERIAL PRIMARY KEY,
    collection_id INTEGER REFERENCES collections(id) ON DELETE CASCADE,
    edition_id    INTEGER REFERENCES editions(id) ON DELETE CASCADE,
    sort_order    INTEGER DEFAULT 0,
    added_at      TIMESTAMP DEFAULT NOW(),
    UNIQUE(collection_id, edition_id)
);

-- ============================================================
-- СТРУКТУРА КНИГ И ЧТЕНИЕ
-- ============================================================

-- Оглавление (для сравнения книг и поиска дубликатов, навигации)
CREATE TABLE toc_entries (
    id          SERIAL PRIMARY KEY,
    edition_id  INTEGER NOT NULL REFERENCES editions(id) ON DELETE CASCADE,
    parent_id   INTEGER REFERENCES toc_entries(id) ON DELETE CASCADE,
    level       INTEGER NOT NULL DEFAULT 1,
    title       TEXT NOT NULL,
    position    INTEGER,                     -- номер параграфа или смещение в файле
    sort_order  INTEGER NOT NULL DEFAULT 0
);

-- Прогресс чтения
CREATE TABLE reading_progress (
    id              SERIAL PRIMARY KEY,
    edition_id      INTEGER UNIQUE REFERENCES editions(id) ON DELETE CASCADE,
    current_position INTEGER DEFAULT 0,
    total_positions  INTEGER,
    percentage       REAL DEFAULT 0,
    device           VARCHAR(100),
    started_at       TIMESTAMP,
    finished_at      TIMESTAMP,
    rating           INTEGER CHECK (rating >= 1 AND rating <= 5),
    notes            TEXT,
    updated_at       TIMESTAMP DEFAULT NOW()
);

-- ============================================================
-- ПОИСК ДУБЛИКАТОВ
-- ============================================================

CREATE TABLE duplicate_candidates (
    id            SERIAL PRIMARY KEY,
    edition_id_1  INTEGER NOT NULL REFERENCES editions(id) ON DELETE CASCADE,
    edition_id_2  INTEGER NOT NULL REFERENCES editions(id) ON DELETE CASCADE,
    match_type    VARCHAR(50) NOT NULL,       -- exact_isbn, file_hash, title_author, toc_similar
    confidence    REAL DEFAULT 0.0,           -- 0.0 до 1.0
    details       JSONB,                      -- дополнительная информация о совпадении
    is_confirmed  BOOLEAN DEFAULT false,
    is_merged     BOOLEAN DEFAULT false,      -- объединены в одну work
    created_at    TIMESTAMP DEFAULT NOW(),
    resolved_at   TIMESTAMP,
    
    UNIQUE(edition_id_1, edition_id_2, match_type)
);

-- ============================================================
-- ИМПОРТ И КОНВЕРТАЦИЯ
-- ============================================================

-- Сессии импорта
CREATE TABLE import_sessions (
    id              SERIAL PRIMARY KEY,
    source_type     VARCHAR(50) NOT NULL,     -- inpx, calibre, manual, directory
    source_path     TEXT,
    started_at      TIMESTAMP DEFAULT NOW(),
    finished_at     TIMESTAMP,
    total_processed INTEGER DEFAULT 0,
    new_works       INTEGER DEFAULT 0,
    new_editions    INTEGER DEFAULT 0,
    new_files       INTEGER DEFAULT 0,
    duplicates_found INTEGER DEFAULT 0,
    errors          TEXT[],
    status          VARCHAR(20) DEFAULT 'running'  -- running, completed, failed
);

-- Лог конвертаций
CREATE TABLE conversion_log (
    id              SERIAL PRIMARY KEY,
    source_file_id  INTEGER REFERENCES edition_files(id) ON DELETE SET NULL,
    target_file_id  INTEGER REFERENCES edition_files(id) ON DELETE SET NULL,
    converter       VARCHAR(50) NOT NULL,
    options         JSONB,
    started_at      TIMESTAMP DEFAULT NOW(),
    finished_at     TIMESTAMP,
    status          VARCHAR(20) DEFAULT 'running',
    error_message   TEXT,
    original_size   BIGINT,
    result_size     BIGINT
);

-- ============================================================
-- ИНДЕКСЫ ДЛЯ ПРОИЗВОДИТЕЛЬНОСТИ
-- ============================================================

-- Триграмные индексы для нечёткого поиска названий и авторов
CREATE INDEX IF NOT EXISTS idx_works_title_trgm      ON works      USING gin (original_title gin_trgm_ops);
CREATE INDEX IF NOT EXISTS idx_editions_title_trgm   ON editions   USING gin (title gin_trgm_ops);
CREATE INDEX IF NOT EXISTS idx_persons_last_trgm     ON persons    USING gin (last_name gin_trgm_ops);
CREATE INDEX IF NOT EXISTS idx_persons_first_trgm    ON persons    USING gin (first_name gin_trgm_ops);
CREATE INDEX IF NOT EXISTS idx_series_trgm           ON editions   USING gin (series gin_trgm_ops);

-- Полнотекстовый поиск
ALTER TABLE editions ADD COLUMN IF NOT EXISTS search_vector tsvector;
CREATE INDEX IF NOT EXISTS idx_editions_fts ON editions USING gin(search_vector);

ALTER TABLE works ADD COLUMN IF NOT EXISTS search_vector tsvector;
CREATE INDEX IF NOT EXISTS idx_works_fts ON works USING gin(search_vector);

-- Индексы для FK и частых запросов
CREATE INDEX IF NOT EXISTS idx_editions_work        ON editions(work_id);
CREATE INDEX IF NOT EXISTS idx_editions_language    ON editions(language);
CREATE INDEX IF NOT EXISTS idx_editions_year        ON editions(year);
CREATE INDEX IF NOT EXISTS idx_editions_isbn        ON editions(isbn) WHERE isbn IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_edition_files_edition ON edition_files(edition_id);
CREATE INDEX IF NOT EXISTS idx_edition_files_format  ON edition_files(format_id);
CREATE INDEX IF NOT EXISTS idx_edition_files_hash    ON edition_files(file_hash) WHERE file_hash IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_work_contributors_work   ON work_contributors(work_id);
CREATE INDEX IF NOT EXISTS idx_work_contributors_person ON work_contributors(person_id);
CREATE INDEX IF NOT EXISTS idx_toc_edition           ON toc_entries(edition_id);
CREATE INDEX IF NOT EXISTS idx_reading_progress_rating ON reading_progress(rating) WHERE rating IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_duplicate_candidates_unconfirmed ON duplicate_candidates(edition_id_1) WHERE is_confirmed = false;

-- Для полнотекстового поиска по toc
CREATE INDEX IF NOT EXISTS idx_toc_title_trgm ON toc_entries USING gin(title gin_trgm_ops);

-- ============================================================
-- ТРИГГЕРЫ ДЛЯ АВТОМАТИЗАЦИИ
-- ============================================================

-- Автообновление updated_at
CREATE OR REPLACE FUNCTION update_timestamp()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_works_updated ON works;
CREATE TRIGGER trg_works_updated    BEFORE UPDATE ON works    FOR EACH ROW EXECUTE FUNCTION update_timestamp();
DROP TRIGGER IF EXISTS trg_editions_updated ON editions;
CREATE TRIGGER trg_editions_updated BEFORE UPDATE ON editions FOR EACH ROW EXECUTE FUNCTION update_timestamp();
DROP TRIGGER IF EXISTS trg_files_updated ON edition_files;
CREATE TRIGGER trg_files_updated    BEFORE UPDATE ON edition_files FOR EACH ROW EXECUTE FUNCTION update_timestamp();

-- Функция для заполнения полей поиска в нижнем регистре (ё→е)
CREATE OR REPLACE FUNCTION normalize_search_field()
RETURNS TRIGGER AS $$
BEGIN
    IF TG_OP = 'INSERT' OR TG_OP = 'UPDATE' THEN
        IF TG_TABLE_NAME = 'persons' THEN
            NEW.lower_fio := REPLACE(LOWER(COALESCE(NEW.last_name, '') || ' ' || COALESCE(NEW.first_name, '')), 'ё', 'е');
        ELSIF TG_TABLE_NAME = 'works' THEN
            NEW.lower_original_title := REPLACE(LOWER(NEW.original_title), 'ё', 'е');
        ELSIF TG_TABLE_NAME = 'editions' THEN
            NEW.lower_title := REPLACE(LOWER(NEW.title), 'ё', 'е');
        END IF;
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_persons_normalize ON persons;
CREATE TRIGGER trg_persons_normalize BEFORE INSERT OR UPDATE ON persons FOR EACH ROW EXECUTE FUNCTION normalize_search_field();

DROP TRIGGER IF EXISTS trg_works_normalize ON works;
CREATE TRIGGER trg_works_normalize BEFORE INSERT OR UPDATE ON works FOR EACH ROW EXECUTE FUNCTION normalize_search_field();

DROP TRIGGER IF EXISTS trg_editions_normalize ON editions;
CREATE TRIGGER trg_editions_normalize BEFORE INSERT OR UPDATE ON editions FOR EACH ROW EXECUTE FUNCTION normalize_search_field();

-- ============================================================
-- ПРЕДСТАВЛЕНИЯ ДЛЯ УДОБСТВА
-- ============================================================

-- Детальная информация о книге
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
    e.updated_at
FROM works w
JOIN editions e ON e.work_id = w.id
LEFT JOIN reading_progress rp ON rp.edition_id = e.id;

-- Сводка по форматам
CREATE OR REPLACE VIEW format_summary AS
SELECT
    f.name,
    f.extension,
    f.category,
    COUNT(ef.id)               AS file_count,
    COUNT(DISTINCT ef.edition_id) AS unique_editions,
    SUM(ef.file_size)          AS total_size_bytes,
    ROUND(AVG(ef.file_size)::numeric, 0) AS avg_size_bytes
FROM formats f
LEFT JOIN edition_files ef ON ef.format_id = f.id
GROUP BY f.id, f.name, f.extension, f.category
ORDER BY file_count DESC;

-- ============================================================
-- ТЕСТОВЫЕ ДАННЫЕ
-- ============================================================

-- Добавляем тестовые данные для быстрой проверки работы системы
INSERT INTO languages (code, name, native_name) VALUES
('rus', 'Russian',  'Русский'),
('eng', 'English',  'English')
ON CONFLICT (code) DO NOTHING;

INSERT INTO formats (name, extension, mime_type, category, is_reflowable, is_editable) VALUES
('EPUB',  'epub',  'application/epub+zip',                                    'ebook',    true,  false),
('PDF',   'pdf',   'application/pdf',                                         'document', false, false)
ON CONFLICT (name) DO NOTHING;

-- Тестовое произведение
WITH inserted_work AS (
    INSERT INTO works (original_title, original_language, first_published, work_type, annotation, word_count)
    VALUES ('Test Book', 'eng', 2023, 'novel', 'This is a test book for the library system', 50000)
    RETURNING id
)
-- Тестовое издание
, inserted_edition AS (
    INSERT INTO editions (work_id, title, language, publisher, year, city, pages, series, annotation, quality, source)
    SELECT id, 'Test Book Edition', 'eng', 'Test Publisher', 2023, 'Test City', 300, 'Test Series', 'Test annotation', 'good', 'test'
    FROM inserted_work
    RETURNING id
)
-- Тестовый файл
INSERT INTO edition_files (edition_id, format_id, file_path, file_size, file_hash, page_count, word_count, has_ocr, has_bookmarks, has_images, is_drm, is_primary, converter)
SELECT e.id, f.id, '/books/test_book.epub', 1024000, 'a665b4516205240608ea134ae5f10c6d8ed290bb27b22d34b9b347e3b411f32a', 300, 50000, false, true, true, false, true, 'calibre'
FROM inserted_edition e
CROSS JOIN (SELECT id FROM formats WHERE name = 'EPUB') f
ON CONFLICT DO NOTHING;

-- Добавляем еще несколько тестовых записей для лучшего тестирования
INSERT INTO works (original_title, original_language, first_published, work_type, annotation, word_count)
SELECT 'Another Test Book', 'eng', 2022, 'novel', 'Another test book', 30000
WHERE NOT EXISTS (SELECT 1 FROM works WHERE original_title = 'Another Test Book');

INSERT INTO editions (work_id, title, language, publisher, year, city, pages, series, annotation, quality, source)
SELECT w.id, 'Another Test Book Edition', 'eng', 'Another Publisher', 2022, 'Another City', 250, 'Another Series', 'Another annotation', 'excellent', 'test'
FROM works w
WHERE w.original_title = 'Another Test Book'
AND NOT EXISTS (SELECT 1 FROM editions WHERE title = 'Another Test Book Edition');

INSERT INTO edition_files (edition_id, format_id, file_path, file_size, file_hash, page_count, word_count, has_ocr, has_bookmarks, has_images, is_drm, is_primary, converter)
SELECT e.id, f.id, '/books/another_test_book.pdf', 2048000, 'b665b4516205240608ea134ae5f10c6d8ed290bb27b22d34b9b347e3b411f32b', 250, 30000, true, false, true, false, true, 'calibre'
FROM editions e
JOIN works w ON e.work_id = w.id
JOIN formats f ON f.name = 'PDF'
WHERE e.title = 'Another Test Book Edition'
AND NOT EXISTS (SELECT 1 FROM edition_files WHERE file_path = '/books/another_test_book.pdf');
