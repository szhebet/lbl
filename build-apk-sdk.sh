#!/bin/bash
# Build the Android SDK builder image (cached, rarely changes)
# Run once or when you need to update Android SDK / Gradle version
set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
cd "${SCRIPT_DIR}"

echo "=== Building Android SDK builder image ==="
echo "This image contains JDK 17 + Android SDK 34 + Gradle 8.5"
echo "It is cached and reused for APK builds."
echo ""

docker build -t library-app-android-sdk:latest -f Dockerfile.android.sdk .

echo ""
echo "=== SDK image ready ==="
docker images library-app-android-sdk:latest --format "Size: {{.Size}}"
echo ""
echo "Now you can build APKs with:"
echo "  ./build-apk-debug.sh"
echo "  ./build-apk-release.sh"
