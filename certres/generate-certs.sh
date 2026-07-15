#!/bin/bash
# Generate self-signed CA + server certificate for HTTPS
set -e

CERTS_DIR="$(cd "$(dirname "$0")" && pwd)"
DAYS=3650
COUNTRY="RU"
STATE="Russia"
LOCALITY="Moscow"
ORG="Home Library"
CA_CN="Home Library CA"
SERVER_CN="library-app.local"

echo "=== Generating Certificate Authority ==="
openssl genrsa -out "${CERTS_DIR}/ca.key" 4096
openssl req -x509 -new -nodes -key "${CERTS_DIR}/ca.key" \
  -sha256 -days "${DAYS}" \
  -subj "/C=${COUNTRY}/ST=${STATE}/L=${LOCALITY}/O=${ORG}/CN=${CA_CN}" \
  -out "${CERTS_DIR}/ca.crt"

echo "=== Generating Server Certificate ==="
openssl genrsa -out "${CERTS_DIR}/server.key" 2048
openssl req -new -key "${CERTS_DIR}/server.key" \
  -subj "/C=${COUNTRY}/ST=${STATE}/L=${LOCALITY}/O=${ORG}/CN=${SERVER_CN}" \
  -out "${CERTS_DIR}/server.csr"

cat > "${CERTS_DIR}/server.ext" << EOF
authorityKeyIdentifier=keyid,issuer
basicConstraints=CA:FALSE
keyUsage = digitalSignature, nonRepudiation, keyEncipherment, dataEncipherment
subjectAltName = @alt_names

[alt_names]
DNS.1 = ${SERVER_CN}
DNS.2 = localhost
IP.1 = 127.0.0.1
IP.2 = 192.168.0.0
EOF

openssl x509 -req -in "${CERTS_DIR}/server.csr" \
  -CA "${CERTS_DIR}/ca.crt" -CAkey "${CERTS_DIR}/ca.key" \
  -CAcreateserial -out "${CERTS_DIR}/server.crt" \
  -days "${DAYS}" -sha256 -extfile "${CERTS_DIR}/server.ext"

# Cleanup
rm -f "${CERTS_DIR}/server.csr" "${CERTS_DIR}/server.ext"

echo "=== Generating PKCS12 for server (for Go TLS) ==="
openssl pkcs12 -export -in "${CERTS_DIR}/server.crt" \
  -inkey "${CERTS_DIR}/server.key" \
  -out "${CERTS_DIR}/server.p12" -passout pass:changeit

echo ""
echo "Certificates generated in ${CERTS_DIR}:"
echo "  ca.crt       - CA certificate (bundle into APK as raw resource)"
echo "  ca.key       - CA private key (keep secure)"
echo "  server.crt   - Server certificate (for Go TLS)"
echo "  server.key   - Server private key (for Go TLS)"
echo "  server.p12   - PKCS12 bundle (for Go TLS)"
echo ""
echo "Next steps:"
echo "  1. Run generate-keystore.sh to create the Android signing key"
echo "  2. Configure main.go to use server.crt + server.key for TLS"
