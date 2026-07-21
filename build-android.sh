#!/bin/bash
# Build Android APK in Docker (debug + release)
# Configuration read from .apk.conf at project root
set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
cd "${SCRIPT_DIR}"

# ── Load configuration ──────────────────────────────────────────
APK_CONF=".apk.conf"
if [ ! -f "$APK_CONF" ]; then
    echo "ERROR: $APK_CONF not found — copy from .apk.conf.example or create one"
    exit 1
fi
source "$APK_CONF"

echo "=== Building Android APK ==="
echo "  Target URL: $APK_TARGET_URL"
echo ""

# ── Step 1: Build SDK image if not cached ──────────────────────
if ! docker image inspect library-app-android-sdk:latest >/dev/null 2>&1; then
    echo "--- Building SDK image (first run, downloads Android SDK) ---"
    docker build -t library-app-android-sdk:latest -f Dockerfile.android.sdk .
fi

# ── Step 2: Generate client certificate if needed ───────────────
if [ ! -f certres/client.crt ]; then
    echo "--- Generating client certificate ---"
    cd certres && bash generate-client-cert.sh && cd ..
fi

# ── Step 3: Copy certificates for Docker build ─────────────────
echo "--- Copying certificates ---"
mkdir -p src_android/app/src/main/res/raw
mkdir -p src_android/certres
cp "$APK_CA_CERT_PATH" src_android/app/src/main/res/raw/ca_cert.crt 2>/dev/null || true
cp "$APK_CLIENT_P12_PATH" src_android/app/src/main/res/raw/client_cert.p12 2>/dev/null || true
cp "$APK_KEYSTORE_PATH" src_android/certres/ 2>/dev/null || true

# ── Step 4: Generate Config.java from .apk.conf ─────────────────
echo "--- Generating Config.java ---"
mkdir -p src_android/app/src/main/java/app/library/twa
cat > src_android/app/src/main/java/app/library/twa/Config.java << JAVA
package app.library.twa;

public final class Config {
    public static final String TARGET_URL = "${APK_TARGET_URL}";
    public static final String CLIENT_CERT_PASSWORD = "${APK_CLIENT_CERT_PASSWORD}";
    private Config() {}
}
JAVA

# ── Step 5: Generate build-extras.gradle from .apk.conf ─────────
echo "--- Generating build-extras.gradle ---"
cat > src_android/build-extras.gradle << GRADLE
ext {
    apkApplicationId = "${APK_APPLICATION_ID}"
    apkVersionCode = ${APK_VERSION_CODE}
    apkVersionName = "${APK_VERSION_NAME}"
    apkCompileSdk = ${APK_COMPILE_SDK}
    apkMinSdk = ${APK_MIN_SDK}
    apkTargetSdk = ${APK_TARGET_SDK}
    apkKeystorePath = rootProject.file('${APK_KEYSTORE_PATH}')
    apkKeystorePassword = "${APK_KEYSTORE_PASSWORD}"
    apkKeyAlias = "${APK_KEY_ALIAS}"
    apkKeyPassword = "${APK_KEY_PASSWORD}"
}
GRADLE

# ── Step 5.5: Copy static files to APK assets for offline use ──
echo ""
echo "--- Copying static files to assets ---"
mkdir -p src_android/app/src/main/assets/www/static/css
mkdir -p src_android/app/src/main/assets/www/static/js
cp static/css/*.css src_android/app/src/main/assets/www/static/css/ 2>/dev/null || true
cp static/js/*.js src_android/app/src/main/assets/www/static/js/ 2>/dev/null || true
cp static/favicon.* src_android/app/src/main/assets/www/static/ 2>/dev/null || true
cp static/service-worker.js src_android/app/src/main/assets/www/ 2>/dev/null || true
cp templates/index.html src_android/app/src/main/assets/www/
cp templates/admin.html src_android/app/src/main/assets/www/
echo "BUILD_$(date +%Y%m%d_%H%M%S)" > src_android/app/src/main/assets/www/VERSION
echo "  $(ls static/css/*.css static/js/*.js | wc -l) files copied"

# ── Step 6: Build APK in Docker ─────────────────────────────────
echo ""
echo "--- Building APK in Docker ---"
docker build -t library-app-android -f Dockerfile.android .

# ── Step 8: Extract APKs ───────────────────────────────────────
echo ""
echo "--- Extracting APKs ---"
mkdir -p android-apk
docker create --name lib-android-tmp library-app-android
docker cp lib-android-tmp:/output/. ./android-apk/ 2>/dev/null || true
docker rm lib-android-tmp

# ── Step 9: Cleanup generated files ────────────────────────────
echo "--- Cleaning up ---"
rm -rf src_android/app/src/main/res/raw/ca_cert.crt \
       src_android/app/src/main/res/raw/client_cert.p12 2>/dev/null
rm -rf src_android/certres 2>/dev/null
rm -f src_android/app/src/main/java/app/library/twa/Config.java src_android/build-extras.gradle

echo ""
echo "=== Build Complete ==="
ls -lh android-apk/*.apk 2>/dev/null && echo "" || echo "  No APKs found"
echo "Install:"
echo "  adb install -r android-apk/app-debug.apk"
