#!/bin/bash
# Build TWA APK (Docker, using cached SDK image)
# For quick builds use: ./build-apk-debug.sh or ./build-apk-release.sh
set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
cd "${SCRIPT_DIR}"

# Ensure SDK builder image exists
if ! docker image inspect library-app-android-sdk:latest >/dev/null 2>&1; then
    echo "SDK builder image not found. Building it first..."
    ./build-apk-sdk.sh
fi

echo "=== Building TWA APK in Docker (debug + release) ==="

# Ensure CA cert is in the android raw resources
mkdir -p src_android/app/src/main/res/raw
cp certres/ca.crt src_android/app/src/main/res/raw/ca_cert.crt 2>/dev/null || true

docker build -t library-app-android:latest -f Dockerfile.android .

rm -rf android-apk && mkdir -p android-apk
docker create --name lib-android-tmp library-app-android:latest
docker cp lib-android-tmp:/output/. ./android-apk/ 2>/dev/null || true
docker rm lib-android-tmp

echo ""
echo "=== APKs ==="
ls -lh android-apk/*.apk 2>/dev/null || echo "No APK found"
echo ""
echo "Install debug:  adb install -r android-apk/app-debug.apk"
echo "Install release: adb install -r android-apk/app-release.apk"
