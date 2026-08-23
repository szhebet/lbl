-- Migration 5.4: record the offered book and delivery state on inbound
-- federated requests. When an admin offers a local book back to the requesting
-- server, the delivery result ("sent"/"delivered"/"failed") and the offered
-- book details are stored so the «Запросы соседей» tab and the offer form can
-- show what was already proposed and whether the message reached the neighbour.

ALTER TABLE fed_incoming_requests ADD COLUMN IF NOT EXISTS offered_edition_id INT;
ALTER TABLE fed_incoming_requests ADD COLUMN IF NOT EXISTS offered_title TEXT NOT NULL DEFAULT '';
ALTER TABLE fed_incoming_requests ADD COLUMN IF NOT EXISTS offered_authors TEXT NOT NULL DEFAULT '';
ALTER TABLE fed_incoming_requests ADD COLUMN IF NOT EXISTS delivery_status TEXT NOT NULL DEFAULT '';
ALTER TABLE fed_incoming_requests ADD COLUMN IF NOT EXISTS delivery_error TEXT NOT NULL DEFAULT '';
ALTER TABLE fed_incoming_requests ADD COLUMN IF NOT EXISTS delivered_at TIMESTAMP;