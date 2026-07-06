#!/bin/bash
# Build the web application and TWA APK independently
set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
cd "${SCRIPT_DIR}"

echo "=== Building library-app (web) + TWA APK ==="

# Step 1: Generate certificates if needed
if [ ! -f certres/android.keystore ] || [ ! -f certres/ca.crt ]; then
    echo ""
    echo "--- Generating certificates ---"
    cd certres
    chmod +x *.sh
    [ ! -f ca.crt ]            && ./generate-certs.sh        || echo "certs already exist"
    [ ! -f android.keystore ]  && ./generate-keystore.sh     || echo "keystore already exists"
    ./generate-assetlinks.sh 2>/dev/null || true
    cd "${SCRIPT_DIR}"
fi

# Step 2: Build web app Docker image
echo ""
echo "--- Building web application image ---"
docker build -t library-app:latest -f Dockerfile .

# Step 3: Build TWA APK
echo ""
echo "--- Building TWA APK ---"
docker build -t library-app-android:latest -f Dockerfile.android .

# Step 4: Extract APK
echo ""
echo "--- Extracting APK ---"
rm -rf android-apk
mkdir -p android-apk
docker create --name lib-android-tmp library-app-android:latest
docker cp lib-android-tmp:/output/. ./android-apk/ 2>/dev/null || true
docker rm lib-android-tmp

# Step 5: Show results
echo ""
echo "=== Build Complete ==="
ls -lh android-apk/*.apk 2>/dev/null && echo "" || echo "  No APK found in artifacts"
echo "Web app image:  library-app:latest"
echo "APK location:   android-apk/"
echo ""
echo "Install APK:"
echo "  adb install android-apk/app-release.apk"
echo ""
echo "Run web app:"
echo "  docker run -d --name library-app --net=host \\"
echo "    -v \${PWD}/config.toml:/config.toml \\"
echo "    -v \${PWD}/bookarch:/bookarch \\"
echo "    library-app:latest"
