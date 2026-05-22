#!/bin/bash
set -e

echo "Starting PostgreSQL..."
docker-entrypoint.sh postgres &
POSTGRES_PID=$!

echo "Waiting for PostgreSQL to be ready..."
until pg_isready -h localhost -p 5432 -U "${POSTGRES_USER}" > /dev/null 2>&1; do
    sleep 1
done
echo "PostgreSQL is ready!"

echo "Starting Library App..."
exec ./library_app