#!/bin/bash
# Build Android SDK Docker image (cached layer)
# Run once, or when Android SDK/Gradle versions change
set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
cd "${SCRIPT_DIR}"

echo "=== Building Android SDK image ==="
docker build -t library-app-android-sdk:latest -f Dockerfile.android.sdk .

echo ""
echo "SDK image built: library-app-android-sdk:latest"
echo "Now run ./build-android.sh to build APKs"
