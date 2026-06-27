#include "mascot.h"
#include "mascot_anim.h"
#include <M5Unified.h>
#include <pgmspace.h>

static uint16_t frameBuffer[MASCOT_PIXELS];

static inline bool keyframe_pixel(int idx) {
    uint8_t b = pgm_read_byte(&MASCOT_KEYFRAME[idx >> 3]);
    return (b >> (7 - (idx & 7))) & 1;
}

void mascot_init() {
    for (int i = 0; i < MASCOT_PIXELS; ++i) {
        frameBuffer[i] = keyframe_pixel(i) ? 0xFFFF : 0x0000;
    }
}

void mascot_draw(int frame, int x, int y) {
    // Rebuild the current frame from the keyframe plus all deltas up to it.
    for (int i = 0; i < MASCOT_PIXELS; ++i) {
        frameBuffer[i] = keyframe_pixel(i) ? 0xFFFF : 0x0000;
    }

    for (int f = 0; f < frame; ++f) {
        uint16_t start = pgm_read_byte((const uint8_t*)&MASCOT_DELTA_OFFSETS[f])
                       | (pgm_read_byte((const uint8_t*)&MASCOT_DELTA_OFFSETS[f] + 1) << 8);
        uint16_t end   = pgm_read_byte((const uint8_t*)&MASCOT_DELTA_OFFSETS[f + 1])
                       | (pgm_read_byte((const uint8_t*)&MASCOT_DELTA_OFFSETS[f + 1] + 1) << 8);
        for (uint16_t p = start; p < end; p += 3) {
            int idx = pgm_read_byte(&MASCOT_DELTA_DATA[p])
                    | (pgm_read_byte(&MASCOT_DELTA_DATA[p + 1]) << 8);
            uint8_t val = pgm_read_byte(&MASCOT_DELTA_DATA[p + 2]);
            frameBuffer[idx] = val ? 0xFFFF : 0x0000;
        }
    }

    M5.Display.pushImage(x, y, MASCOT_IMG_W, MASCOT_IMG_H, frameBuffer, true);
}
