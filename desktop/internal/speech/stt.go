package speech

// STTEngine transcribes audio into text.
type STTEngine interface {
	Transcribe(audioData []byte, format string) (string, error)
}
