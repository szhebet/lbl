-- ============================================================
-- Миграция 2.0
-- Все дальнейшие изменения схемы БД вносить сюда,
-- увеличивая версию при необходимости.
-- ============================================================

-- Таблица устройств пользователей
CREATE TABLE IF NOT EXISTS user_devices (
    id                 SERIAL PRIMARY KEY,
    user_id            INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    device_name        VARCHAR(255) NOT NULL,
    device_fingerprint VARCHAR(255) NOT NULL,
    created_at         TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(device_name),
    UNIQUE(device_fingerprint)
);
