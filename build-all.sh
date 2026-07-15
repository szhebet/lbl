#!/bin/bash
# Build the web application Docker image
set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
cd "${SCRIPT_DIR}"

echo "=== Building library-app (web) ==="

# Step 1: Generate certificates if needed
if [ ! -f certres/android.keystore ] || [ ! -f certres/ca.crt ]; then
    echo ""
    echo "--- Generating certificates ---"
    cd certres
    chmod +x *.sh
    [ ! -f ca.crt ]            && ./generate-certs.sh        || echo "certs already exist"
    [ ! -f android.keystore ]  && ./generate-keystore.sh     || echo "keystore already exists"
    cd "${SCRIPT_DIR}"
fi

# Step 2: Build web app Docker image
echo ""
echo "--- Building web application image ---"
docker build -t library-app:latest -f Dockerfile .

echo ""
echo "=== Build Complete ==="
echo "Web app image:  library-app:latest"
echo ""
echo "Run web app:"
echo "  docker run -d --name library-app --net=host \\"
echo "    -v \${PWD}/config.toml:/config.toml \\"
echo "    -v \${PWD}/bookarch:/bookarch \\"
echo "    -v \${PWD}/tempfld:/tempfld \\"
echo "    -v \${PWD}/logs:/logs \\"
echo "    -v \${PWD}/templates:/templates \\"
echo "    -v \${PWD}/static:/static \\"
echo "    library-app:latest /library_app"
