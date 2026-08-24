-- ============================================================
-- Миграция 5.5 — независимые UUID-идентификаторы сущностей.
-- Каждому произведению (works), персоне (persons) и изданию
-- (editions) добавляется стабильный колонковый uid, используемый
-- для кросс-серверного матчинга в федерации. Локальные числовые
-- ключи (serial) и все их FK остаются без изменений.
-- ============================================================

ALTER TABLE works ADD COLUMN IF NOT EXISTS uid UUID;
UPDATE works SET uid = gen_random_uuid() WHERE uid IS NULL;
ALTER TABLE works ALTER COLUMN uid SET DEFAULT gen_random_uuid();
ALTER TABLE works ALTER COLUMN uid SET NOT NULL;
CREATE UNIQUE INDEX IF NOT EXISTS idx_works_uid ON works(uid);

ALTER TABLE persons ADD COLUMN IF NOT EXISTS uid UUID;
UPDATE persons SET uid = gen_random_uuid() WHERE uid IS NULL;
ALTER TABLE persons ALTER COLUMN uid SET DEFAULT gen_random_uuid();
ALTER TABLE persons ALTER COLUMN uid SET NOT NULL;
CREATE UNIQUE INDEX IF NOT EXISTS idx_persons_uid ON persons(uid);

ALTER TABLE editions ADD COLUMN IF NOT EXISTS uid UUID;
UPDATE editions SET uid = gen_random_uuid() WHERE uid IS NULL;
ALTER TABLE editions ALTER COLUMN uid SET DEFAULT gen_random_uuid();
ALTER TABLE editions ALTER COLUMN uid SET NOT NULL;
CREATE UNIQUE INDEX IF NOT EXISTS idx_editions_uid ON editions(uid);