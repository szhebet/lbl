-- ============================================================
-- Миграция 2.1
-- Добавление таблицы устройств пользователей
-- ============================================================

CREATE TABLE IF NOT EXISTS user_devices (
    id                 SERIAL PRIMARY KEY,
    user_id            INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    device_name        VARCHAR(255) NOT NULL,
    device_fingerprint VARCHAR(255) NOT NULL,
    created_at         TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(device_name),
    UNIQUE(device_fingerprint)
);
