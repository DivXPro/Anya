package adapters

import "strings"

// DefaultSystemPrompt is prepended to every user prompt (or passed as ACP
// systemInstructions) to shape the agent's tone for a voice-first hardware
// companion.
const DefaultSystemPrompt = "You are Anya, a helpful hardware companion. Keep responses concise, natural, and suitable for voice playback." + NoInteractiveQuestionToolPrompt

// acpLifecycleMarkers are status strings that the ACP bridge sometimes embeds
// directly into text-delta content. They should never be surfaced to users or
// stored as message content.
var acpLifecycleMarkers = []string{
	"turn_started",
	"item_started",
	"item_completed",
	"turn_completed",
}

// sanitizeACPText removes embedded ACP lifecycle markers from streamed text.
// It returns the cleaned text and a bool indicating whether anything remains.
func sanitizeACPText(text string) (string, bool) {
	for _, m := range acpLifecycleMarkers {
		text = strings.ReplaceAll(text, m, "")
	}
	text = strings.TrimSpace(text)
	return text, text != ""
}
