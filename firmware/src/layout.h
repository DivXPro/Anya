#pragma once

// Shared screen layout constants for the main idle screen and the WiFi portal.
// The display is 135x240 portrait. The status bar sits at the top, the mascot
// is centred vertically but shifted upward, and the prompt text sits below it.

static const int STATUS_BAR_H    = 16;
static const int MASCOT_GAP      = 24;  // space between mascot and prompt text
static const int MASCOT_TOP_BIAS = -16; // shift mascot up from vertical centre
static const int PROMPT_H        = 12;

// Compute mascotY / promptY for a screen of the given height and a mascot of
// the given height. Both screens use the same layout algorithm.
inline void computeMascotLayout(int screenH, int mascotH, int& mascotY, int& promptY) {
    mascotY = STATUS_BAR_H +
              (screenH - STATUS_BAR_H - mascotH - PROMPT_H - MASCOT_GAP) / 2 +
              MASCOT_TOP_BIAS;
    promptY = mascotY + mascotH + MASCOT_GAP;
}
