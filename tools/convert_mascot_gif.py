#!/usr/bin/env python3
"""Convert an animated GIF to a compact delta-encoded grayscale mascot for Elf firmware.

Output is a header + source pair (e.g. mascot_anim.h / mascot_anim.cpp).
- Frame 0 is stored as a full packed keyframe.
- Subsequent frames are stored as a list of changed pixels relative to the
  previous frame, wrapped back to frame 0 at the end of the loop.
- Each delta entry is 3 bytes: uint16_t pixel offset + uint8_t new value.

All timing is in milliseconds.
"""

import sys
from pathlib import Path
from PIL import Image

SCRIPT_DIR = Path(__file__).resolve().parent
INPUT = Path(sys.argv[1]) if len(sys.argv) > 1 else SCRIPT_DIR / "../desktop/frontend/public/cat.gif"
OUTPUT_H = Path(sys.argv[2]) if len(sys.argv) > 2 else SCRIPT_DIR / "../firmware/src/mascot_anim.h"
OUTPUT_CPP = OUTPUT_H.with_suffix(".cpp")
MAX_FRAMES = int(sys.argv[3]) if len(sys.argv) > 3 else 200
CROP_W = int(sys.argv[4]) if len(sys.argv) > 4 else None
CROP_H = int(sys.argv[5]) if len(sys.argv) > 5 else None
BITS_PER_PIXEL = 2  # 1 = black/white, 2 = 4 gray levels, 4 = 16 gray levels
DEFAULT_FRAME_MS = 40
LAST_FRAME_HOLD_MS = 2000


def frames_equal(a: Image.Image, b: Image.Image) -> bool:
    return a.size == b.size and list(a.getdata()) == list(b.getdata())


def quantize_frame(frame: Image.Image) -> Image.Image:
    """Quantize a frame to BITS_PER_PIXEL grayscale levels (0..2^bits-1)."""
    levels = 1 << BITS_PER_PIXEL
    shift = 8 - BITS_PER_PIXEL
    return frame.convert("L").point(lambda p: min(p >> shift, levels - 1), mode="L")


def downsample(frames, durations, max_frames):
    """Limit to the first max_frames while preserving each frame's timing."""
    if len(frames) <= max_frames:
        return frames, durations
    return frames[:max_frames], durations[:max_frames]


def pack_pixels(values, bpp):
    """Pack a list of pixel values (0..2^bpp-1) into bytes, MSB first."""
    pixels_per_byte = 8 // bpp
    mask = (1 << bpp) - 1
    n = len(values)
    out = bytearray((n + pixels_per_byte - 1) // pixels_per_byte)
    for i, v in enumerate(values):
        byte_idx = i // pixels_per_byte
        bit_shift = 8 - bpp - (i % pixels_per_byte) * bpp
        out[byte_idx] |= (v & mask) << bit_shift
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


def format_array_u32(name, data, width=8):
    lines = [f"const uint32_t {name}[] PROGMEM = {{"]
    row = "    "
    for i, v in enumerate(data):
        row += f"0x{v:08X}, "
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
            # Composite onto black, then quantize to grayscale levels.
            frame = gif.convert("RGBA")
            bg = Image.new("RGBA", frame.size, (0, 0, 0, 255))
            composite = Image.alpha_composite(bg, frame)
            raw_frames.append(quantize_frame(composite))
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

    frames, durations = downsample(dedup_frames, dedup_durations, MAX_FRAMES)
    if durations:
        durations[-1] = max(durations[-1], LAST_FRAME_HOLD_MS)

    if CROP_W and CROP_H:
        frames = [f.crop((0, 0, min(CROP_W, f.width), min(CROP_H, f.height))) for f in frames]

    w, h = frames[0].size
    frame_count = len(frames)

    # Convert to pixel value lists.
    bits_list = [list(f.getdata()) for f in frames]

    # Keyframe = frame 0.
    keyframe_bytes = pack_pixels(bits_list[0], BITS_PER_PIXEL)

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
            delta_data.append(value & 0xFF)
        delta_offsets.append(len(delta_data))

    header_lines = [
        "// Auto-generated mascot animation data.",
        f"// Source: {INPUT}",
        f"// {frame_count} frames @ {w}x{h}, {BITS_PER_PIXEL}-bit keyframe + delta-encoded changes.",
        "#pragma once",
        "#include <cstdint>",
        "",
        f"constexpr int MASCOT_IMG_W = {w};",
        f"constexpr int MASCOT_IMG_H = {h};",
        f"constexpr int MASCOT_FRAMES = {frame_count};",
        f"constexpr int MASCOT_PIXELS = {w * h};",
        f"constexpr int MASCOT_BITS_PER_PIXEL = {BITS_PER_PIXEL};",
        f"constexpr int MASCOT_LEVELS = {1 << BITS_PER_PIXEL};",
        "",
        "extern const int MASCOT_FRAME_DURATIONS[MASCOT_FRAMES];",
        f"extern const uint8_t MASCOT_KEYFRAME[{len(keyframe_bytes)}];",
        f"extern const uint32_t MASCOT_DELTA_OFFSETS[{frame_count + 1}];",
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
    cpp_lines += format_array_u32("MASCOT_DELTA_OFFSETS", delta_offsets)
    cpp_lines.append("")
    cpp_lines += format_array_u8("MASCOT_DELTA_DATA", delta_data)
    cpp_lines.append("")

    with open(OUTPUT_H, "w") as f:
        f.write("\n".join(header_lines))
    with open(OUTPUT_CPP, "w") as f:
        f.write("\n".join(cpp_lines))

    print(f"Generated {OUTPUT_H} / {OUTPUT_CPP}")
    print(f"Frames: {frame_count} @ {w}x{h}, {BITS_PER_PIXEL}-bit, keyframe {len(keyframe_bytes)} bytes, delta {len(delta_data)} bytes")
    print(f"Durations (ms): {durations}")


if __name__ == "__main__":
    main()
