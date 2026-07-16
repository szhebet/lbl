-- Migration 4.0: Settings and backup infrastructure
CREATE TABLE IF NOT EXISTS settings (
    key         VARCHAR(100) PRIMARY KEY,
    value       TEXT NOT NULL DEFAULT '',
    updated_at  TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

INSERT INTO settings (key, value) VALUES
    ('backup_dir', '')
ON CONFLICT (key) DO NOTHING;
