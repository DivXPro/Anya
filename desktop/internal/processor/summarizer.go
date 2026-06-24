package processor

import (
	"strings"
)

func Summarize(content string) (string, error) {
	sentence := extractFirstSentence(content)
	if len([]rune(sentence)) > 50 {
		return truncate(sentence, 50), nil
	}
	return sentence, nil
}

func extractFirstSentence(text string) string {
	for _, sep := range []string{"。", "！", "？", ". ", "!\n", "?\n", "\n\n"} {
		if idx := strings.Index(text, sep); idx > 0 {
			return strings.TrimSpace(text[:idx+len(sep)])
		}
	}
	return strings.TrimSpace(text)
}
