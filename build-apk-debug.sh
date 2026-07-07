#!/bin/bash
# Build only the debug TWA APK in Docker
# Uses cached library-app-android-sdk image for the build environment
set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
cd "${SCRIPT_DIR}"

# Ensure SDK builder image exists
if ! docker image inspect library-app-android-sdk:latest >/dev/null 2>&1; then
    echo "SDK builder image not found. Building it first..."
    ./build-apk-sdk.sh
fi

echo "=== Building debug APK ==="
# Generate client certificate if needed
./certres/generate-client-cert.sh

# Ensure CA cert + client cert are in the android raw resources
mkdir -p src_android/app/src/main/res/raw
cp certres/ca.crt src_android/app/src/main/res/raw/ca_cert.crt 2>/dev/null || true
cp certres/client.p12 src_android/app/src/main/res/raw/client_cert.p12 2>/dev/null || true

docker build --no-cache -t library-app-android:latest -f Dockerfile.android .

rm -rf android-apk && mkdir -p android-apk
docker create --name lib-android-tmp library-app-android:latest
docker cp lib-android-tmp:/output/app-debug.apk ./android-apk/ 2>/dev/null || \
  docker cp lib-android-tmp:/output/. ./android-apk/
docker rm lib-android-tmp

echo ""
echo "=== Debug APK ==="
ls -lh android-apk/*debug*.apk 2>/dev/null || ls -lh android-apk/*.apk 2>/dev/null || echo "No APK found"
echo ""
echo "Install: adb install -r android-apk/app-debug.apk"
