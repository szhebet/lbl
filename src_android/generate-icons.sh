#!/bin/bash
# Generate PNG launcher icons for legacy Android versions (pre-26).
# Requires ImageMagick or Inkscape.
set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
RES_DIR="${SCRIPT_DIR}/app/src/main/res"
FOREGROUND="${RES_DIR}/drawable/ic_launcher_foreground.xml"

echo "=== Generating launcher icons ==="

# Check for tools
if command -v convert &> /dev/null; then
    TOOL="convert"
elif command -v inkscape &> /dev/null; then
    TOOL="inkscape"
else
    echo "Warning: Neither ImageMagick nor Inkscape found."
    echo "Skipping PNG icon generation. Adaptive icons (API 26+) will be used."
    echo "Install ImageMagick: apt install imagemagick"
    exit 0
fi

if [ "$TOOL" = "convert" ]; then
    echo "Using ImageMagick"

    # Generate a simple book icon using ImageMagick
    SIZES=(
        "mipmap-hdpi:48"
        "mipmap-mdpi:36"
        "mipmap-xhdpi:64"
        "mipmap-xxhdpi:96"
        "mipmap-xxxhdpi:128"
    )

    for entry in "${SIZES[@]}"; do
        DIR="${entry%%:*}"
        SIZE="${entry##*:}"
        ICON="${RES_DIR}/${DIR}/ic_launcher.png"

        convert -size "${SIZE}x${SIZE}" xc:'#1A1A2E' \
            -fill '#3498DB' -draw "roundrectangle 4,4,$((SIZE-5)),$((SIZE-5)) 6,6" \
            -fill '#FFFFFF' -draw "rectangle $((SIZE/4)),$((SIZE/3)) $((SIZE*3/4)),$((SIZE*2/3-4))" \
            -fill '#2ECC71' -draw "polygon $((SIZE/4)),$((SIZE*2/3)) $((SIZE/2)),$((SIZE*2/3-8)) $((SIZE*3/4)),$((SIZE*2/3))" \
            "${ICON}"

        echo "  Created ${DIR}/ic_launcher.png (${SIZE}x${SIZE})"
    done
fi

echo "Done!"
