#!/bin/bash
set -e
CERTS_DIR="$(cd "$(dirname "$0")" && pwd)"
cp "${CERTS_DIR}/server.crt" "${CERTS_DIR}/fullchain.pem"
cp "${CERTS_DIR}/server.key" "${CERTS_DIR}/privkey.pem"
echo "Generated ${CERTS_DIR}/fullchain.pem and ${CERTS_DIR}/privkey.pem for nginx"
