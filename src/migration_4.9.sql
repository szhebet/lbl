-- Migration 4.9: API neighbours (peer library servers).
--
-- Each row describes a trusted remote library server that this instance
-- can connect to: its base URL, the TLS server certificate to trust during
-- the handshake, an optional client certificate for mutual TLS, and the
-- username/password used for authentication. The password is stored
-- encrypted (AES-256-GCM); the key lives in the settings table and is
-- generated on first use by the application.
CREATE TABLE IF NOT EXISTS api_neighbours (
    id                 SERIAL PRIMARY KEY,
    url                VARCHAR(500) NOT NULL,
    server_cert        TEXT NOT NULL DEFAULT '',
    client_cert        TEXT NOT NULL DEFAULT '',
    username           VARCHAR(255) NOT NULL DEFAULT '',
    password_encrypted TEXT NOT NULL DEFAULT '',
    created_at         TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at         TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
