#!/usr/bin/env python3
"""
Create Chrome extension icons with Q logo for Quaero.
Generates 16x16, 48x48, and 128x128 PNG icons.
"""

import os
import struct
import zlib

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
    """Draw a Q icon at the given size."""
    # Colors: Blue background (#0d6efd), White Q (#ffffff)
    blue = [13, 110, 253, 255]  # RGBA
    white = [255, 255, 255, 255]

    pixels = []
    center = size / 2
    outer_radius = size * 0.4
    inner_radius = size * 0.22
    stroke_width = size * 0.12

    for y in range(size):
        for x in range(size):
            # Distance from center
            dx = x - center + 0.5
            dy = y - center + 0.5
            dist = (dx * dx + dy * dy) ** 0.5

            # Default to background
            color = blue

            # Draw circle outline (Q body)
            if inner_radius + stroke_width < dist < outer_radius:
                color = white

            # Draw Q tail (diagonal line from center-right going down-right)
            tail_start_x = center + inner_radius * 0.5
            tail_start_y = center + inner_radius * 0.5
            tail_end_x = center + outer_radius * 1.1
            tail_end_y = center + outer_radius * 1.1

            # Check if point is on the tail line
            if x >= tail_start_x - stroke_width/2 and y >= tail_start_y - stroke_width/2:
                if x <= tail_end_x and y <= tail_end_y:
                    # Distance from diagonal line y = x (relative to tail start)
                    rel_x = x - tail_start_x
                    rel_y = y - tail_start_y
                    line_dist = abs(rel_x - rel_y) / (2 ** 0.5)
                    if line_dist < stroke_width * 0.7:
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
        print(f"Creating {size}x{size} icon...")
        pixels = draw_q_icon(size)
        png_data = create_png(size, size, pixels)

        output_path = os.path.join(icons_dir, f'icon{size}.png')
        with open(output_path, 'wb') as f:
            f.write(png_data)
        print(f"  Created: {output_path}")

    print("\nExtension icons created successfully!")


if __name__ == '__main__':
    main()
