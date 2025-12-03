#!/usr/bin/env python3
"""
Create Chrome extension icons matching the web favicon design.
Blue circle background (#0d6efd) with white Q letter.
Generates 16x16, 48x48, and 128x128 PNG icons.
"""

import os
import struct
import zlib
import math

def create_png(width, height, pixels):
    """Create a PNG file from RGBA pixel data."""
    def png_chunk(chunk_type, data):
        chunk = chunk_type + data
        return struct.pack('>I', len(data)) + chunk + struct.pack('>I', zlib.crc32(chunk) & 0xffffffff)

    # PNG signature
    signature = b'\x89PNG\r\n\x1a\n'

    # IHDR chunk
    ihdr_data = struct.pack('>IIBBBBB', width, height, 8, 6, 0, 0, 0)  # 8-bit RGBA
    ihdr = png_chunk(b'IHDR', ihdr_data)

    # IDAT chunk (image data)
    raw_data = b''
    for y in range(height):
        raw_data += b'\x00'  # Filter type: None
        for x in range(width):
            idx = (y * width + x) * 4
            raw_data += bytes(pixels[idx:idx+4])

    compressed = zlib.compress(raw_data, 9)
    idat = png_chunk(b'IDAT', compressed)

    # IEND chunk
    iend = png_chunk(b'IEND', b'')

    return signature + ihdr + idat + iend


def draw_q_icon(size):
    """
    Draw a Q icon matching the web favicon design:
    - Blue circle background (#0d6efd)
    - White Q letter centered
    """
    # Colors: Blue background (#0d6efd), White Q (#ffffff)
    blue = [13, 110, 253, 255]  # RGBA - #0d6efd
    white = [255, 255, 255, 255]
    transparent = [0, 0, 0, 0]

    pixels = []
    center = size / 2
    circle_radius = size * 0.48  # Match SVG: r="48" out of viewBox="100"

    # Q letter parameters (scaled from SVG font-size="60" in viewBox="100")
    q_scale = size / 100.0

    # Font metrics for Q (approximate)
    font_size = 60 * q_scale
    stroke_width = font_size * 0.15  # Letter stroke width

    # Q circle parameters
    q_outer_radius = font_size * 0.38
    q_inner_radius = q_outer_radius - stroke_width

    # Q tail parameters
    tail_length = font_size * 0.35
    tail_width = stroke_width * 1.2
    tail_angle = math.radians(45)  # 45 degrees

    for y in range(size):
        for x in range(size):
            # Distance from center
            dx = x - center + 0.5
            dy = y - center + 0.5
            dist = math.sqrt(dx * dx + dy * dy)

            # Default to transparent (outside circle)
            color = transparent

            # Check if inside the blue circle background
            if dist <= circle_radius:
                color = blue  # Blue background

                # Check if this pixel is part of the white Q letter
                is_q = False

                # Q body (circle outline)
                if q_inner_radius < dist < q_outer_radius:
                    is_q = True

                # Q tail (diagonal line from bottom-right of circle)
                # Tail starts at roughly 45 degrees and extends outward
                tail_start_x = center + q_inner_radius * 0.5
                tail_start_y = center + q_inner_radius * 0.5

                # Check if point is on the tail
                rel_x = x - tail_start_x + 0.5
                rel_y = y - tail_start_y + 0.5

                if rel_x >= 0 and rel_y >= 0:
                    # Distance along the diagonal
                    diag_dist = (rel_x + rel_y) / math.sqrt(2)
                    # Perpendicular distance from diagonal
                    perp_dist = abs(rel_x - rel_y) / math.sqrt(2)

                    if diag_dist < tail_length and perp_dist < tail_width / 2:
                        is_q = True

                if is_q:
                    color = white

            pixels.extend(color)

    return pixels


def main():
    script_dir = os.path.dirname(os.path.abspath(__file__))
    project_root = os.path.dirname(script_dir)
    icons_dir = os.path.join(project_root, 'cmd', 'quaero-chrome-extension', 'icons')

    os.makedirs(icons_dir, exist_ok=True)

    sizes = [16, 48, 128]

    for size in sizes:
        print(f"Creating {size}x{size} icon (matching web favicon)...")
        pixels = draw_q_icon(size)
        png_data = create_png(size, size, pixels)

        output_path = os.path.join(icons_dir, f'icon{size}.png')
        with open(output_path, 'wb') as f:
            f.write(png_data)
        print(f"  Created: {output_path}")

    print("\nExtension icons created successfully!")
    print("Design: Blue circle (#0d6efd) with white Q letter")


if __name__ == '__main__':
    main()
