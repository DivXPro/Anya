#!/usr/bin/env python3
"""Convert an animated GIF to a compact delta-encoded 1-bit mascot for Elf firmware.

Output is a header + source pair (e.g. mascot_anim.h / mascot_anim.cpp).
- Frame 0 is stored as a full 1-bit keyframe.
- Subsequent frames are stored as a list of changed pixels relative to the
  previous frame, wrapped back to frame 0 at the end of the loop.
- Each delta entry is 3 bytes: uint16_t pixel offset + uint8_t new value.

All timing is in milliseconds.
"""

import sys
from pathlib import Path
from PIL import Image

SCRIPT_DIR = Path(__file__).resolve().parent
INPUT = Path(sys.argv[1]) if len(sys.argv) > 1 else SCRIPT_DIR / "../desktop/frontend/public/cat-loop-1.gif"
OUTPUT_H = Path(sys.argv[2]) if len(sys.argv) > 2 else SCRIPT_DIR / "../firmware/src/mascot_anim.h"
OUTPUT_CPP = OUTPUT_H.with_suffix(".cpp")
MAX_FRAMES = int(sys.argv[3]) if len(sys.argv) > 3 else 12
DEFAULT_FRAME_MS = 40
LAST_FRAME_HOLD_MS = 2000
BW_THRESHOLD = 128  # Pixels lighter than this become white.


def frames_equal(a: Image.Image, b: Image.Image) -> bool:
    return a.size == b.size and list(a.getdata()) == list(b.getdata())


def threshold_frame(frame: Image.Image) -> Image.Image:
    """Return a 1-bit image with only pure black/white pixels."""
    gray = frame.convert("L")
    return gray.point(lambda p: 255 if p > BW_THRESHOLD else 0, mode="1")


def downsample(frames, durations):
    """Evenly sample frames to at most MAX_FRAMES while preserving timing."""
    if len(frames) <= MAX_FRAMES:
        return frames, durations
    step = len(frames) / MAX_FRAMES
    out_frames = []
    out_durations = []
    for i in range(MAX_FRAMES):
        start = int(round(i * step))
        end = int(round((i + 1) * step))
        end = min(end, len(frames))
        idx = min((start + end) // 2, len(frames) - 1)
        out_frames.append(frames[idx])
        out_durations.append(sum(durations[start:end]))
    return out_frames, out_durations


def bits_to_bytes(bits):
    """Pack a list of 0/1 ints into MSB-first bytes."""
    n = len(bits)
    out = bytearray((n + 7) // 8)
    for i, b in enumerate(bits):
        if b:
            out[i >> 3] |= 0x80 >> (i & 7)
    return bytes(out)


def encode_deltas(prev_bits, cur_bits):
    """Return list of (pixel_offset, new_value) for changed pixels."""
    return [(i, cur) for i, (prev, cur) in enumerate(zip(prev_bits, cur_bits)) if prev != cur]


def format_array_u8(name, data, width=16):
    lines = [f"const uint8_t {name}[] PROGMEM = {{"]
    row = "    "
    for i, b in enumerate(data):
        row += f"0x{b:02X}, "
        if (i + 1) % width == 0:
            lines.append(row.rstrip())
            row = "    "
    if row.strip():
        lines.append(row.rstrip().rstrip(","))
    lines.append("};")
    return lines


def format_array_u16(name, data, width=12):
    lines = [f"const uint16_t {name}[] PROGMEM = {{"]
    row = "    "
    for i, v in enumerate(data):
        row += f"0x{v:04X}, "
        if (i + 1) % width == 0:
            lines.append(row.rstrip())
            row = "    "
    if row.strip():
        lines.append(row.rstrip().rstrip(","))
    lines.append("};")
    return lines


def main() -> None:
    gif = Image.open(INPUT)
    raw_frames = []

    try:
        while True:
            # Composite onto black, then threshold to pure black/white.
            frame = gif.convert("RGBA")
            bg = Image.new("RGBA", frame.size, (0, 0, 0, 255))
            composite = Image.alpha_composite(bg, frame)
            raw_frames.append(threshold_frame(composite))
            gif.seek(gif.tell() + 1)
    except EOFError:
        pass

    if not raw_frames:
        raise RuntimeError(f"No frames found in {INPUT}")

    # Deduplicate consecutive identical frames.
    dedup_frames = [raw_frames[0]]
    dedup_durations = [DEFAULT_FRAME_MS]
    for frame in raw_frames[1:]:
        if frames_equal(frame, dedup_frames[-1]):
            dedup_durations[-1] += DEFAULT_FRAME_MS
        else:
            dedup_frames.append(frame)
            dedup_durations.append(DEFAULT_FRAME_MS)

    frames, durations = downsample(dedup_frames, dedup_durations)
    if durations:
        durations[-1] = max(durations[-1], LAST_FRAME_HOLD_MS)

    w, h = frames[0].size
    frame_count = len(frames)

    # Convert to bit lists.
    bits_list = [list(f.getdata()) for f in frames]

    # Keyframe = frame 0.
    keyframe_bytes = bits_to_bytes(bits_list[0])

    # Delta data: for each transition i -> (i+1) % frame_count.
    delta_data = bytearray()
    delta_offsets = [0]
    for i in range(frame_count):
        prev = bits_list[i]
        cur = bits_list[(i + 1) % frame_count]
        changes = encode_deltas(prev, cur)
        for offset, value in changes:
            delta_data.append(offset & 0xFF)
            delta_data.append((offset >> 8) & 0xFF)
            delta_data.append(1 if value else 0)
        delta_offsets.append(len(delta_data))

    header_lines = [
        "// Auto-generated mascot animation data.",
        f"// Source: {INPUT}",
        f"// {frame_count} frames @ {w}x{h}, 1-bit keyframe + delta-encoded changes.",
        "#pragma once",
        "#include <cstdint>",
        "",
        f"constexpr int MASCOT_IMG_W = {w};",
        f"constexpr int MASCOT_IMG_H = {h};",
        f"constexpr int MASCOT_FRAMES = {frame_count};",
        f"constexpr int MASCOT_PIXELS = {w * h};",
        "",
        "extern const int MASCOT_FRAME_DURATIONS[MASCOT_FRAMES];",
        f"extern const uint8_t MASCOT_KEYFRAME[{len(keyframe_bytes)}];",
        f"extern const uint16_t MASCOT_DELTA_OFFSETS[{frame_count + 1}];",
        "extern const uint8_t MASCOT_DELTA_DATA[];",
        "",
    ]

    cpp_lines = [
        f'// Auto-generated mascot animation data.',
        f'// Source: {INPUT}',
        '#include "mascot_anim.h"',
        '#include <pgmspace.h>',
        "",
    ]

    # Durations array.
    cpp_lines += [
        f"const int MASCOT_FRAME_DURATIONS[MASCOT_FRAMES] PROGMEM = {{",
        "    " + ", ".join(str(d) for d in durations),
        "};",
        "",
    ]

    cpp_lines += format_array_u8("MASCOT_KEYFRAME", keyframe_bytes)
    cpp_lines.append("")
    cpp_lines += format_array_u16("MASCOT_DELTA_OFFSETS", delta_offsets)
    cpp_lines.append("")
    cpp_lines += format_array_u8("MASCOT_DELTA_DATA", delta_data)
    cpp_lines.append("")

    with open(OUTPUT_H, "w") as f:
        f.write("\n".join(header_lines))
    with open(OUTPUT_CPP, "w") as f:
        f.write("\n".join(cpp_lines))

    print(f"Generated {OUTPUT_H} / {OUTPUT_CPP}")
    print(f"Frames: {frame_count} @ {w}x{h}, keyframe {len(keyframe_bytes)} bytes, delta {len(delta_data)} bytes")
    print(f"Durations (ms): {durations}")


if __name__ == "__main__":
    main()
