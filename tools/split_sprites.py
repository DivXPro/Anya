#!/usr/bin/env python3
"""Split a sprite sheet image into individual frames and generate C header."""

import sys
import os
from PIL import Image

INPUT = sys.argv[1] if len(sys.argv) > 1 else "spritesheet.png"
COLS, ROWS = 4, 6
OUT_DIR = sys.argv[2] if len(sys.argv) > 2 else "out"
GEN_HEADER = len(sys.argv) > 3  # third arg → generate header

img = Image.open(INPUT)
W, H = img.size
cell_w = W // COLS
cell_h = H // ROWS

print(f"Image: {W}x{H}, cell: {cell_w}x{cell_h} ({W%COLS}x{H%ROWS} leftover)")

os.makedirs(OUT_DIR, exist_ok=True)
code_lines = []
total = 0

# Expressions / states mapped row-major: row 0-5, col 0-3
for row in range(ROWS):
    for col in range(COLS):
        x, y = col * cell_w, row * cell_h
        cell = img.crop((x, y, x + cell_w, y + cell_h))
        # Resize to 80×80 for M5StickC S3 mascot area
        cell_80 = cell.resize((80, 80), Image.LANCZOS)
        fname = f"frame_{row}_{col}.bmp"
        path = os.path.join(OUT_DIR, fname)
        cell_80.save(path)
        total += 1

print(f"Exported {total} frames → {OUT_DIR}/")

if GEN_HEADER:
    # Generate C header with bitmap data for embedding in firmware
    header = os.path.join(OUT_DIR, "mascot_frames.h")
    with open(header, "w") as f:
        f.write("// Auto-generated sprite frames (80×80 RGB565)\n")
        f.write(f"// Source: {INPUT} ({W}x{H})\n\n")
        f.write("#pragma once\n#include <cstdint>\n\n")
        f.write(f"#define MASCOT_FRAME_COUNT {total}\n")
        f.write(f"#define MASCOT_FRAME_W 80\n")
        f.write(f"#define MASCOT_FRAME_H 80\n\n")
        for row in range(ROWS):
            for col in range(COLS):
                fname = f"frame_{row}_{col}.bmp"
                cell = Image.open(os.path.join(OUT_DIR, fname)).convert("RGB")
                pixels = list(cell.getdata())
                name = f"frame_{row}_{col}"
                f.write(f"const uint16_t {name}[] PROGMEM = {{\n    ")
                vals = []
                for r, g, b in pixels:
                    # RGB888 → RGB565
                    rgb565 = ((r >> 3) << 11) | ((g >> 2) << 5) | (b >> 3)
                    vals.append(f"0x{rgb565:04X}")
                for i, v in enumerate(vals):
                    f.write(v)
                    if (i + 1) % 10 == 0:
                        f.write(",\n    ")
                    else:
                        f.write(", ")
                f.write("\n};\n\n")
    print(f"Header: {header}")
