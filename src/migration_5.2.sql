-- Migration 5.2: UID-based delivery accounting for federated requests.
--
-- Every outgoing request gets a UUID (uid) that is transmitted to neighbour
-- servers and used as the dedup key. A message is pushed to a neighbour only
-- if it has not already been delivered to that server (tracked per-neighbour
-- in fed_request_outbox by uid). The receiving server dedups inbound messages
-- by (source_url, uid) and reports "already exists" so the sender can mark its
-- outbox row delivered and avoid resending.

ALTER TABLE fed_outgoing_requests ADD COLUMN IF NOT EXISTS uid UUID;
UPDATE fed_outgoing_requests SET uid = gen_random_uuid() WHERE uid IS NULL;
ALTER TABLE fed_outgoing_requests ALTER COLUMN uid SET NOT NULL;
ALTER TABLE fed_outgoing_requests ALTER COLUMN uid SET DEFAULT gen_random_uuid();

ALTER TABLE fed_incoming_requests ADD COLUMN IF NOT EXISTS uid UUID;
UPDATE fed_incoming_requests SET uid = gen_random_uuid() WHERE uid IS NULL;

ALTER TABLE fed_request_outbox ADD COLUMN IF NOT EXISTS uid UUID;
UPDATE fed_request_outbox SET uid = gen_random_uuid() WHERE uid IS NULL;
ALTER TABLE fed_request_outbox ALTER COLUMN uid SET DEFAULT gen_random_uuid();

-- A message from a given source is identified by its uid; receiving the same
-- uid twice means it was already delivered and must not be stored again.
CREATE UNIQUE INDEX IF NOT EXISTS idx_fed_incoming_source_uid
    ON fed_incoming_requests(source_url, uid) WHERE uid IS NOT NULL;