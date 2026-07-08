#!/bin/bash
# Generate assetlinks.json for Digital Asset Links verification
# Required for Trusted Web Activity (TWA)
set -e

CERTS_DIR="$(cd "$(dirname "$0")" && pwd)"
KEYSTORE="${CERTS_DIR}/android.keystore"
ALIAS="library-app"
STOREPASS="android"

SITE="library-app.local"
PACKAGE_NAME="app.library.twa"

echo "=== Generating assetlinks.json ==="

if [ ! -f "${KEYSTORE}" ]; then
    echo "Error: Keystore not found at ${KEYSTORE}"
    echo "Run generate-keystore.sh first."
    exit 1
fi

SHA256=$(keytool -list -v -keystore "${KEYSTORE}" \
  -storepass "${STOREPASS}" \
  -alias "${ALIAS}" 2>/dev/null \
  | grep "SHA256:" | awk '{print $2}' | tr -d '\n')

if [ -z "${SHA256}" ]; then
    echo "Failed to extract SHA256 fingerprint"
    exit 1
fi

echo "Using SHA256: ${SHA256}"

cat > "${CERTS_DIR}/assetlinks.json" << EOF
[{
  "relation": ["delegate_permission/common.handle_all_urls"],
  "target": {
    "namespace": "android_app",
    "package_name": "${PACKAGE_NAME}",
    "sha256_cert_fingerprints": ["${SHA256}"]
  }
}]
EOF

echo ""
echo "assetlinks.json generated at ${CERTS_DIR}/assetlinks.json"
echo ""
echo "Deploy this file to your web server:"
echo "  mkdir -p /path/to/.well-known"
echo "  cp ${CERTS_DIR}/assetlinks.json /path/to/.well-known/"
echo ""
echo "For the Go app, create a static route:"
echo "  r.GET(\"/.well-known/assetlinks.json\", func(c *gin.Context) {"
echo '    c.File("./certres/assetlinks.json")'
echo "  })"
