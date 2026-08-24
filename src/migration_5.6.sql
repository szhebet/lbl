-- ============================================================
-- Миграция 5.6 — фиксация выполнения запросов по федерации.
-- Когда удалённый сервер отправляет в ответ книгу (offer через
-- /api/v1/server/book/offer), принимающая сторона фиксирует факт
-- получения на строке approved-запроса (fed_outgoing_requests),
-- чтобы администратор видел «книга получена» и с какого сервера.
-- ============================================================

ALTER TABLE fed_outgoing_requests ADD COLUMN IF NOT EXISTS fulfilled_at TIMESTAMPTZ;
ALTER TABLE fed_outgoing_requests ADD COLUMN IF NOT EXISTS fulfilled_by_url TEXT;