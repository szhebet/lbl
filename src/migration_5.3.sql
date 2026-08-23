-- Migration 5.3: stable read_list_id on inbound federated requests.
--
-- The uid of a federated request is regenerated whenever an approved outgoing
-- request is revoked and re-approved, so linking an offered book back to the
-- originating user request by uid alone is unreliable (a peer may hold a stale
-- incoming row with an old uid). To link reliably the requester now propagates
-- the stable read_list id with every pushed request and stores it on the
-- incoming side; the offer carries it back and the receiver links directly.

ALTER TABLE fed_incoming_requests ADD COLUMN IF NOT EXISTS read_list_id UUID;

CREATE INDEX IF NOT EXISTS idx_fed_incoming_read_list_id
    ON fed_incoming_requests(read_list_id);