ALTER TABLE read_list ADD COLUMN deleted BOOLEAN NOT NULL DEFAULT FALSE;
UPDATE db_version SET version = '3.1';
