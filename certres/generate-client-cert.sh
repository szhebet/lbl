#!/bin/bash
# Generate client certificate for nginx mutual TLS authentication
# Reuses existing certificate if already present.
set -e

CERTS_DIR="$(cd "$(dirname "$0")" && pwd)"
DAYS=3650
COUNTRY="RU"
STATE="Russia"
LOCALITY="Moscow"
ORG="Home Library"
CLIENT_CN="library-app-client"
P12_PASSWORD="changeit"

CERT_FILE="${CERTS_DIR}/client.crt"
KEY_FILE="${CERTS_DIR}/client.key"
P12_FILE="${CERTS_DIR}/client.p12"

if [ -f "${CERT_FILE}" ] && [ -f "${KEY_FILE}" ]; then
    echo "=== Client certificate already exists, reusing ==="
    echo "  ${CERT_FILE}"
    echo "  ${KEY_FILE}"
    if [ ! -f "${P12_FILE}" ]; then
        echo "=== PKCS12 bundle missing, regenerating ==="
        openssl pkcs12 -export -in "${CERT_FILE}" \
            -inkey "${KEY_FILE}" \
            -out "${P12_FILE}" -passout pass:"${P12_PASSWORD}"
    fi
    echo "  ${P12_FILE}"
    exit 0
fi

echo "=== Generating Client Certificate ==="

# Generate client private key
openssl genrsa -out "${KEY_FILE}" 2048

# Generate client CSR
openssl req -new -key "${KEY_FILE}" \
    -subj "/C=${COUNTRY}/ST=${STATE}/L=${LOCALITY}/O=${ORG}/CN=${CLIENT_CN}" \
    -out "${CERTS_DIR}/client.csr"

# Sign with CA
cat > "${CERTS_DIR}/client.ext" << EOF
authorityKeyIdentifier=keyid,issuer
basicConstraints=CA:FALSE
keyUsage = digitalSignature, nonRepudiation, keyEncipherment
extendedKeyUsage = clientAuth
subjectAltName = DNS:${CLIENT_CN}
EOF

openssl x509 -req -in "${CERTS_DIR}/client.csr" \
    -CA "${CERTS_DIR}/ca.crt" -CAkey "${CERTS_DIR}/ca.key" \
    -CAcreateserial -out "${CERT_FILE}" \
    -days "${DAYS}" -sha256 -extfile "${CERTS_DIR}/client.ext"

# Cleanup CSR and ext file
rm -f "${CERTS_DIR}/client.csr" "${CERTS_DIR}/client.ext"

# Create PKCS12 bundle for Android
echo "=== Generating PKCS12 for Android ==="
openssl pkcs12 -export -in "${CERT_FILE}" \
    -inkey "${KEY_FILE}" \
    -out "${P12_FILE}" -passout pass:"${P12_PASSWORD}"

echo ""
echo "Client certificate generated in ${CERTS_DIR}:"
echo "  client.crt   - Client certificate (send to nginx for whitelisting)"
echo "  client.key   - Client private key (keep secure)"
echo "  client.p12   - PKCS12 bundle (bundled into APK)"
echo ""
echo "To configure nginx, add to server block:"
echo "  ssl_client_certificate ${CERTS_DIR}/ca.crt;"
echo "  ssl_verify_client on;"
echo ""
echo "To extract the client certificate for whitelisting:"
echo "  Copy ${CERT_FILE} to nginx server and reference in ssl_client_certificate"
