-- Migration 4.6: Add created_by (creator) to read_list
ALTER TABLE read_list ADD COLUMN IF NOT EXISTS created_by INTEGER REFERENCES users(id) ON DELETE SET NULL;

CREATE INDEX IF NOT EXISTS idx_read_list_created_by ON read_list(created_by);
