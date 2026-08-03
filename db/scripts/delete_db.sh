#!/bin/bash
# ============================================================
# Скрипт удаления тестовой БД
# Используется ТОЛЬКО для тестирования, не для релизной поставки
# ============================================================
set -e

CONFIG_FILE="${1:-config.toml}"

# Читаем параметры БД из config.toml (только секцию [database])
DB_HOST=$(sed -n '/^\[database\]/,/^\[/{s/^host = "\(.*\)"/\1/p}' "$CONFIG_FILE" 2>/dev/null || echo "localhost")
DB_PORT=$(sed -n '/^\[database\]/,/^\[/{s/^port = //p}' "$CONFIG_FILE" 2>/dev/null || echo "5432")
DB_NAME=$(sed -n '/^\[database\]/,/^\[/{s/^name = "\(.*\)"/\1/p}' "$CONFIG_FILE" 2>/dev/null || echo "library")
DB_USER=$(sed -n '/^\[database\]/,/^\[/{s/^user = "\(.*\)"/\1/p}' "$CONFIG_FILE" 2>/dev/null || echo "postgres")
DB_PASS=$(sed -n '/^\[database\]/,/^\[/{s/^password = "\(.*\)"/\1/p}' "$CONFIG_FILE" 2>/dev/null || echo "postgres")

echo "Stopping library_app if running..."
pkill -x library_app 2>/dev/null || true
sleep 1

echo "Dropping database '$DB_NAME' on $DB_HOST:$DB_PORT..."
export PGPASSWORD="$DB_PASS"
psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d postgres -c "DROP DATABASE IF EXISTS $DB_NAME;" 2>&1
echo "Database '$DB_NAME' deleted."

# Очищаем директории с данными
rm -rf bookarch/* tempfld/* logs/* tmpBookarch/* 2>/dev/null
mkdir -p bookarch/covers tempfld logs tmpBookarch
echo "Directories cleaned."
