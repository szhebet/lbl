-- Migration 2.4: Triggers for syncing reading status between read_list and user_books
-- Uses pg_trigger_depth() to prevent cascading loops

-- Sync read_list → user_books
CREATE OR REPLACE FUNCTION sync_readlist_status_to_userbooks()
RETURNS TRIGGER AS $$
BEGIN
    IF pg_trigger_depth() > 1 THEN
        RETURN NEW;
    END IF;
    IF NEW.book_id IS NOT NULL AND OLD.status IS DISTINCT FROM NEW.status THEN
        INSERT INTO user_books (user_id, edition_id, status)
        VALUES (NEW.user_id, NEW.book_id, NEW.status)
        ON CONFLICT (user_id, edition_id) DO UPDATE SET
            status = NEW.status,
            updated_at = CURRENT_TIMESTAMP;
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_readlist_sync_status ON read_list;
CREATE TRIGGER trg_readlist_sync_status
AFTER UPDATE OF status ON read_list
FOR EACH ROW EXECUTE FUNCTION sync_readlist_status_to_userbooks();

-- Sync user_books → read_list
CREATE OR REPLACE FUNCTION sync_userbooks_status_to_readlist()
RETURNS TRIGGER AS $$
BEGIN
    IF pg_trigger_depth() > 1 THEN
        RETURN NEW;
    END IF;
    IF OLD.status IS DISTINCT FROM NEW.status THEN
        UPDATE read_list
        SET status = NEW.status
        WHERE user_id = NEW.user_id AND book_id = NEW.edition_id;
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_userbooks_sync_status ON user_books;
CREATE TRIGGER trg_userbooks_sync_status
AFTER UPDATE OF status ON user_books
FOR EACH ROW EXECUTE FUNCTION sync_userbooks_status_to_readlist();
