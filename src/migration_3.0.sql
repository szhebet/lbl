-- Migration 3.0: Read list UUID primary key + timestamps for offline sync
-- Requires PostgreSQL 13+ (gen_random_uuid)

-- Add uuid column, populate for existing rows
ALTER TABLE read_list ADD COLUMN IF NOT EXISTS new_id UUID;
UPDATE read_list SET new_id = gen_random_uuid() WHERE new_id IS NULL;
ALTER TABLE read_list ALTER COLUMN new_id SET NOT NULL;

-- Add unique index on new_id
CREATE UNIQUE INDEX IF NOT EXISTS idx_read_list_new_id ON read_list(new_id);

-- Add timestamp columns for offline sync
ALTER TABLE read_list ADD COLUMN IF NOT EXISTS updated_at TIMESTAMP DEFAULT NOW();
ALTER TABLE read_list ADD COLUMN IF NOT EXISTS synced_at TIMESTAMP;

-- Set updated_at = created_at for existing rows
UPDATE read_list SET updated_at = COALESCE(created_at, NOW()) WHERE updated_at IS NULL;

-- Drop old id SERIAL, rename new_id to id
ALTER TABLE read_list DROP CONSTRAINT IF EXISTS read_list_pkey CASCADE;
ALTER TABLE read_list DROP COLUMN IF EXISTS id;
ALTER TABLE read_list RENAME COLUMN new_id TO id;
ALTER TABLE read_list ADD PRIMARY KEY (id);

-- Index on user_id still exists from previous migration
