#include "mascot.h"
#include "mascot_anim.h"
#include <M5Unified.h>
#include <pgmspace.h>

static uint16_t frameBuffer[MASCOT_PIXELS];
static uint16_t levelToRgb565[MASCOT_LEVELS];

static inline uint8_t keyframe_pixel(int idx) {
    int bits = MASCOT_BITS_PER_PIXEL;
    int pixels_per_byte = 8 / bits;
    int byte_idx = idx / pixels_per_byte;
    int shift = 8 - bits - (idx % pixels_per_byte) * bits;
    uint8_t b = pgm_read_byte(&MASCOT_KEYFRAME[byte_idx]);
    return (b >> shift) & ((1 << bits) - 1);
}

void mascot_init() {
    // Non-linear grayscale mapping: skip the darkest range where this LCD
    // tends to show a purple/blue tint. Level 1 jumps straight to 50% gray.
    static const uint8_t LEVEL_TO_BRIGHTNESS[MASCOT_LEVELS] = {0, 128, 191, 255};
    for (int l = 0; l < MASCOT_LEVELS; ++l) {
        uint8_t v = LEVEL_TO_BRIGHTNESS[l];
        levelToRgb565[l] = ((v >> 3) << 11) | ((v >> 2) << 5) | (v >> 3);
    }
    for (int i = 0; i < MASCOT_PIXELS; ++i) {
        frameBuffer[i] = levelToRgb565[keyframe_pixel(i)];
    }
}

void mascot_draw(int frame, int x, int y) {
    // Rebuild the current frame from the keyframe plus all deltas up to it.
    for (int i = 0; i < MASCOT_PIXELS; ++i) {
        frameBuffer[i] = levelToRgb565[keyframe_pixel(i)];
    }

    for (int f = 0; f < frame; ++f) {
        uint32_t start = pgm_read_byte((const uint8_t*)&MASCOT_DELTA_OFFSETS[f])
                       | (pgm_read_byte((const uint8_t*)&MASCOT_DELTA_OFFSETS[f] + 1) << 8)
                       | (pgm_read_byte((const uint8_t*)&MASCOT_DELTA_OFFSETS[f] + 2) << 16)
                       | (pgm_read_byte((const uint8_t*)&MASCOT_DELTA_OFFSETS[f] + 3) << 24);
        uint32_t end   = pgm_read_byte((const uint8_t*)&MASCOT_DELTA_OFFSETS[f + 1])
                       | (pgm_read_byte((const uint8_t*)&MASCOT_DELTA_OFFSETS[f + 1] + 1) << 8)
                       | (pgm_read_byte((const uint8_t*)&MASCOT_DELTA_OFFSETS[f + 1] + 2) << 16)
                       | (pgm_read_byte((const uint8_t*)&MASCOT_DELTA_OFFSETS[f + 1] + 3) << 24);
        for (uint32_t p = start; p < end; p += 3) {
            int idx = pgm_read_byte(&MASCOT_DELTA_DATA[p])
                    | (pgm_read_byte(&MASCOT_DELTA_DATA[p + 1]) << 8);
            uint8_t val = pgm_read_byte(&MASCOT_DELTA_DATA[p + 2]);
            frameBuffer[idx] = levelToRgb565[val];
        }
    }

    M5.Display.pushImage(x, y, MASCOT_IMG_W, MASCOT_IMG_H, frameBuffer, true);
}
