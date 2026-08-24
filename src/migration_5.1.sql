-- Migration 5.1: Approved outbound book-request staging.
--
-- fed_outgoing_requests is the admin-approved set of user book requests to be
-- advertised to neighbouring library servers. A row is inserted here from a
-- read_list request ONLY after the admin presses «Запросить по федерации» on
-- the "Запросы" tab. The background distributor (fedRequestsDistributor)
-- reads exclusively from this table — it never pushes raw user requests on its
-- own.
--
-- One row per approved user request (UNIQUE read_list_id). status:
--   * approved  – queued for delivery to neighbours (delivered via
--                 fed_request_outbox per neighbour)
--   * removed   – admin revoked approval; distributor ignores it and cancels
--                 the corresponding pending/failed outbox rows.

CREATE TABLE IF NOT EXISTS fed_outgoing_requests (
    id            SERIAL PRIMARY KEY,
    read_list_id  UUID,
    bookname      TEXT NOT NULL DEFAULT '',
    author        TEXT NOT NULL DEFAULT '',
    priority      INT  NOT NULL DEFAULT 0,
    status        TEXT NOT NULL DEFAULT 'approved',
    created_at    TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at    TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_fed_outgoing_readlist
    ON fed_outgoing_requests(read_list_id) WHERE read_list_id IS NOT NULL;