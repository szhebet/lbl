-- ============================================================
-- Миграция 5.7 — журнал предложений книг по федерации.
-- Каждый оффер (POST /api/v1/server/book/offer) для запроса
-- пользователя фиксируется в fed_offers: какая книга, с какого
-- сервера, когда скачана и стала доступна. К запросу (read_list)
-- автоматически привязывается только ПЕРВОЕ предложение; остальные
-- просто импортируются в библиотеку. Пользователь может связать
-- с записью списка чтения любое из предложений вручную.
-- ============================================================

CREATE TABLE IF NOT EXISTS fed_offers (
    id                BIGSERIAL PRIMARY KEY,
    read_list_id      UUID NOT NULL REFERENCES read_list(id) ON DELETE CASCADE,
    source_url        TEXT NOT NULL DEFAULT '',
    remote_work_id    BIGINT NOT NULL DEFAULT 0,
    remote_edition_id BIGINT NOT NULL DEFAULT 0,
    local_edition_id  INTEGER REFERENCES editions(id) ON DELETE SET NULL,
    title             TEXT NOT NULL DEFAULT '',
    authors           TEXT NOT NULL DEFAULT '',
    received_at       TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    linked            BOOLEAN NOT NULL DEFAULT FALSE
);

CREATE INDEX IF NOT EXISTS idx_fed_offers_read_list ON fed_offers(read_list_id);

CREATE UNIQUE INDEX IF NOT EXISTS idx_fed_offers_unique
    ON fed_offers(read_list_id, source_url, remote_edition_id);
