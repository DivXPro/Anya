#!/usr/bin/env python3
"""Convert an animated GIF to an embedded RGB565 mascot header for Elf firmware.

The GIF frames are kept at their original size. Consecutive identical frames are
deduplicated and their durations summed. The last frame is held longer to avoid
constant motion. All timing is in milliseconds.
"""

import sys
from pathlib import Path
from PIL import Image

SCRIPT_DIR = Path(__file__).resolve().parent
INPUT = Path(sys.argv[1]) if len(sys.argv) > 1 else SCRIPT_DIR / "../desktop/frontend/public/cat-loop-1.gif"
OUTPUT = Path(sys.argv[2]) if len(sys.argv) > 2 else SCRIPT_DIR / "../firmware/src/mascot_img.h"
MAX_FRAMES = int(sys.argv[3]) if len(sys.argv) > 3 else 12
DEFAULT_FRAME_MS = 40  # delay 4 (hundredths of a second)
LAST_FRAME_HOLD_MS = 2000
BW_THRESHOLD = 128  # Pixels lighter than this become white; darker become black.


def rgb565(r: int, g: int, b: int) -> int:
    return ((r >> 3) << 11) | ((g >> 2) << 5) | (b >> 3)


def frames_equal(a: Image.Image, b: Image.Image) -> bool:
    return a.size == b.size and list(a.getdata()) == list(b.getdata())


def main() -> None:
    gif = Image.open(INPUT)
    raw_frames = []

    try:
        while True:
            # Convert to RGBA, composite onto black, then reduce to pure black/white.
            # The source GIF is antialiased greyscale; thresholding removes the grey
            # fringe and makes the background truly black on the LCD.
            frame = gif.convert("RGBA")
            bg = Image.new("RGBA", frame.size, (0, 0, 0, 255))
            composite = Image.alpha_composite(bg, frame).convert("L")
            bw = composite.point(lambda p: 255 if p > BW_THRESHOLD else 0, mode="1")
            raw_frames.append(bw.convert("RGB"))
            gif.seek(gif.tell() + 1)
    except EOFError:
        pass

    if not raw_frames:
        raise RuntimeError(f"No frames found in {INPUT}")

    # Deduplicate consecutive identical frames; each source frame is 40 ms.
    dedup_frames = [raw_frames[0]]
    dedup_durations = [DEFAULT_FRAME_MS]
    for frame in raw_frames[1:]:
        if frames_equal(frame, dedup_frames[-1]):
            dedup_durations[-1] += DEFAULT_FRAME_MS
        else:
            dedup_frames.append(frame)
            dedup_durations.append(DEFAULT_FRAME_MS)

    # If there are still too many frames, evenly downsample while preserving
    # overall timing. Each kept frame represents multiple source frames.
    if len(dedup_frames) > MAX_FRAMES:
        step = len(dedup_frames) / MAX_FRAMES
        sampled_frames = []
        sampled_durations = []
        for i in range(MAX_FRAMES):
            start = int(round(i * step))
            end = int(round((i + 1) * step))
            end = min(end, len(dedup_frames))
            # Take the middle frame of the bucket.
            idx = min((start + end) // 2, len(dedup_frames) - 1)
            sampled_frames.append(dedup_frames[idx])
            sampled_durations.append(sum(dedup_durations[start:end]))
        unique_frames = sampled_frames
        unique_durations = sampled_durations
    else:
        unique_frames = dedup_frames
        unique_durations = dedup_durations

    # Make the last frame hold longer to avoid constant motion.
    if unique_durations:
        unique_durations[-1] = max(unique_durations[-1], LAST_FRAME_HOLD_MS)

    w, h = unique_frames[0].size
    frame_count = len(unique_frames)

    lines = [
        "// Auto-generated mascot from animated GIF",
        f"// Source: {INPUT}",
        f"// Frames: {frame_count}, size: {w}x{h}, RGB565",
        "#pragma once",
        "#include <cstdint>",
        "",
        f"constexpr int MASCOT_IMG_W = {w};",
        f"constexpr int MASCOT_IMG_H = {h};",
        f"constexpr int MASCOT_FRAMES = {frame_count};",
        "",
        f"constexpr int MASCOT_FRAME_DURATIONS[MASCOT_FRAMES] = {{",
    ]
    dur_str = ", ".join(str(d) for d in unique_durations)
    lines.append(f"    {dur_str}")
    lines.append("};")
    lines.append("")

    for i, frame in enumerate(unique_frames):
        pixels = list(frame.getdata())
        vals = [f"0x{rgb565(r, g, b):04X}" for r, g, b in pixels]
        lines.append(f"static const uint16_t mascot_frame_{i}[] PROGMEM = {{")
        row = "    "
        for j, v in enumerate(vals):
            row += v + ", "
            if (j + 1) % 8 == 0:
                lines.append(row.rstrip())
                row = "    "
        if row.strip():
            lines.append(row.rstrip().rstrip(","))
        lines.append("};")
        lines.append("")

    lines.append("static const uint16_t* const mascot_frames[MASCOT_FRAMES] PROGMEM = {")
    ptrs = ", ".join(f"mascot_frame_{i}" for i in range(frame_count))
    lines.append(f"    {ptrs}")
    lines.append("};")
    lines.append("")

    with open(OUTPUT, "w") as f:
        f.write("\n".join(lines))

    print(f"Generated {OUTPUT}: {frame_count} frames @ {w}x{h}")
    print(f"Durations (ms): {unique_durations}")


if __name__ == "__main__":
    main()
