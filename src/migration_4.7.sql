-- Migration 4.7: Add ru_name (Russian display name) to genres
ALTER TABLE genres ADD COLUMN IF NOT EXISTS ru_name TEXT;
