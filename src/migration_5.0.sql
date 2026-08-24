-- Migration 5.0: Federated book-request distribution.
--
-- Two tables are introduced:
--   * fed_incoming_requests – book requests received FROM neighbouring library
--     servers via POST /api/v1/server/requests/push. They are shown to the
--     admin on the "Управление" page (Requests from neighbours tab). Each row
--     records the source server URL, the book title/author being requested,
--     an optional priority and a mutable processing status.
--   * fed_request_outbox – outbound delivery bookkeeping for requests this
--     server pushes TO its neighbours. Row = (neighbour, requested title,
--     author). status: pending -> delivered (ack) / failed; attempts counts
--     delivery tries; next_retry_at schedules the next attempt; last_error
--     keeps the most recent failure message. Persisting this lets delivery +
--     retries survive process restarts.

CREATE TABLE IF NOT EXISTS fed_incoming_requests (
    id          SERIAL PRIMARY KEY,
    source_url  VARCHAR(500) NOT NULL DEFAULT '',
    bookname    TEXT NOT NULL DEFAULT '',
    author      TEXT NOT NULL DEFAULT '',
    priority    INT  NOT NULL DEFAULT 0,
    status      TEXT NOT NULL DEFAULT 'new',
    created_at  TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at  TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_fed_incoming_status ON fed_incoming_requests(status);

CREATE TABLE IF NOT EXISTS fed_request_outbox (
    id            SERIAL PRIMARY KEY,
    neighbour_id  INT NOT NULL REFERENCES api_neighbours(id) ON DELETE CASCADE,
    bookname      TEXT NOT NULL DEFAULT '',
    author        TEXT NOT NULL DEFAULT '',
    priority      INT  NOT NULL DEFAULT 0,
    status        TEXT NOT NULL DEFAULT 'pending',
    attempts      INT  NOT NULL DEFAULT 0,
    next_retry_at TIMESTAMP,
    last_error    TEXT NOT NULL DEFAULT '',
    created_at    TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at    TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- One outbox row per neighbour per requested title (author) is sufficient.
CREATE UNIQUE INDEX IF NOT EXISTS idx_fed_outbox_neighbour_book
    ON fed_request_outbox(neighbour_id, bookname, author);