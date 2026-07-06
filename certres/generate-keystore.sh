#!/bin/bash
# Generate Android signing keystore for TWA APK
set -e

CERTS_DIR="$(cd "$(dirname "$0")" && pwd)"
KEYSTORE="${CERTS_DIR}/android.keystore"
ALIAS="library-app"
KEYALG="RSA"
KEYSIZE=2048
VALIDITY=10000
STOREPASS="android"
KEYPASS="android"
DNAME="CN=Home Library, OU=Development, O=Library, L=Moscow, S=Russia, C=RU"

echo "=== Generating Android Signing Keystore ==="

if [ -f "${KEYSTORE}" ]; then
    echo "Keystore already exists at ${KEYSTORE}"
    echo "Delete it first if you want to regenerate."
    exit 1
fi

keytool -genkey -v \
  -keystore "${KEYSTORE}" \
  -alias "${ALIAS}" \
  -keyalg "${KEYALG}" \
  -keysize "${KEYSIZE}" \
  -validity "${VALIDITY}" \
  -storepass "${STOREPASS}" \
  -keypass "${KEYPASS}" \
  -dname "${DNAME}"

echo ""
echo "Keystore generated: ${KEYSTORE}"
echo "  Store password: ${STOREPASS}"
echo "  Key password:   ${KEYPASS}"
echo "  Alias:          ${ALIAS}"
echo ""

# Extract SHA256 fingerprint for assetlinks.json
echo "=== SHA256 Fingerprint (for Digital Asset Links) ==="
keytool -list -v -keystore "${KEYSTORE}" \
  -storepass "${STOREPASS}" \
  -alias "${ALIAS}" 2>/dev/null \
  | grep "SHA256:" | awk '{print $2}' | tr -d '\n'
echo ""
echo ""
echo "Save this fingerprint to use in generate-assetlinks.sh"
