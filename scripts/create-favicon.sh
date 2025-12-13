#!/bin/bash
# Create favicon.ico file for Quaero
# Creates a blue background with white Q

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"

# Output paths
STATIC_DIR="$PROJECT_ROOT/pages/static"
PAGES_DIR="$PROJECT_ROOT/pages"

mkdir -p "$STATIC_DIR"

# Create favicon.ico using hex data
# 16x16 icon with blue (#0d6efd) background and white Q
create_favicon() {
    local output_file="$1"

    # ICO file header + 16x16 32-bit BGRA image
    # Blue background: #0d6efd -> BGRA: fd 6e 0d ff
    # White: #ffffff -> BGRA: ff ff ff ff

    printf '\x00\x00'       # Reserved
    printf '\x01\x00'       # Type (ICO)
    printf '\x01\x00'       # Image count

    # Directory entry
    printf '\x10'           # Width (16)
    printf '\x10'           # Height (16)
    printf '\x00'           # Colors (0 = true color)
    printf '\x00'           # Reserved
    printf '\x01\x00'       # Planes
    printf '\x20\x00'       # Bits per pixel (32)
    printf '\x68\x04\x00\x00'  # Image size (1128 bytes)
    printf '\x16\x00\x00\x00'  # Offset (22)

    # BMP header
    printf '\x28\x00\x00\x00'  # Header size (40)
    printf '\x10\x00\x00\x00'  # Width (16)
    printf '\x20\x00\x00\x00'  # Height (32, doubled for ICO)
    printf '\x01\x00'          # Planes
    printf '\x20\x00'          # Bits per pixel (32)
    printf '\x00\x00\x00\x00'  # Compression
    printf '\x00\x04\x00\x00'  # Image size
    printf '\x00\x00\x00\x00'  # X pixels/meter
    printf '\x00\x00\x00\x00'  # Y pixels/meter
    printf '\x00\x00\x00\x00'  # Colors used
    printf '\x00\x00\x00\x00'  # Important colors

    # Pixel data (16x16, bottom-up, BGRA)
    # Pattern: Blue circle background with white Q letter
    local blue='\xfd\x6e\x0d\xff'
    local white='\xff\xff\xff\xff'

    # Row 15 (bottom) to Row 0 (top)
    # Q pattern centered at (8,8) with outer radius ~6, inner ~3

    # Rows 15-13: bottom edge
    for row in 15 14 13; do
        for col in $(seq 0 15); do
            printf "$blue"
        done
    done

    # Row 12: Q tail starts
    for col in $(seq 0 15); do
        if [ $col -ge 10 ] && [ $col -le 12 ]; then
            printf "$white"
        else
            printf "$blue"
        fi
    done

    # Row 11: Q bottom curve + tail
    for col in $(seq 0 15); do
        if ([ $col -ge 5 ] && [ $col -le 10 ]) || ([ $col -ge 11 ] && [ $col -le 13 ]); then
            printf "$white"
        else
            printf "$blue"
        fi
    done

    # Row 10: Q bottom
    for col in $(seq 0 15); do
        if [ $col -ge 4 ] && [ $col -le 11 ]; then
            printf "$white"
        else
            printf "$blue"
        fi
    done

    # Row 9: Q sides
    for col in $(seq 0 15); do
        if ([ $col -ge 3 ] && [ $col -le 5 ]) || ([ $col -ge 10 ] && [ $col -le 12 ]); then
            printf "$white"
        else
            printf "$blue"
        fi
    done

    # Row 8: Q sides (middle)
    for col in $(seq 0 15); do
        if ([ $col -ge 3 ] && [ $col -le 5 ]) || ([ $col -ge 10 ] && [ $col -le 12 ]); then
            printf "$white"
        else
            printf "$blue"
        fi
    done

    # Row 7: Q sides
    for col in $(seq 0 15); do
        if ([ $col -ge 3 ] && [ $col -le 5 ]) || ([ $col -ge 10 ] && [ $col -le 12 ]); then
            printf "$white"
        else
            printf "$blue"
        fi
    done

    # Row 6: Q sides
    for col in $(seq 0 15); do
        if ([ $col -ge 3 ] && [ $col -le 5 ]) || ([ $col -ge 10 ] && [ $col -le 12 ]); then
            printf "$white"
        else
            printf "$blue"
        fi
    done

    # Row 5: Q top curve
    for col in $(seq 0 15); do
        if [ $col -ge 4 ] && [ $col -le 11 ]; then
            printf "$white"
        else
            printf "$blue"
        fi
    done

    # Row 4: Q top
    for col in $(seq 0 15); do
        if [ $col -ge 5 ] && [ $col -le 10 ]; then
            printf "$white"
        else
            printf "$blue"
        fi
    done

    # Rows 3-0: top edge
    for row in 3 2 1 0; do
        for col in $(seq 0 15); do
            printf "$blue"
        done
    done

    # AND mask (64 bytes of zeros for transparency)
    for i in $(seq 1 64); do
        printf '\x00'
    done
}

echo "Creating favicon.ico..."
create_favicon > "$STATIC_DIR/favicon.ico"
cp "$STATIC_DIR/favicon.ico" "$PAGES_DIR/favicon.ico"

echo "Favicon created at:"
echo "  - $STATIC_DIR/favicon.ico"
echo "  - $PAGES_DIR/favicon.ico"
