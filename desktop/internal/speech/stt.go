package speech

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type STTEngine interface {
	Transcribe(audioData []byte, format string) (string, error)
}

type WhisperSTT struct {
	model    string
	language string
	tmpDir   string
}

func NewWhisperSTT(model, language string) *WhisperSTT {
	return &WhisperSTT{
		model:    model,
		language: language,
		tmpDir:   filepath.Join("elf-data", "tmp"),
	}
}

func (w *WhisperSTT) Transcribe(audioData []byte, format string) (string, error) {
	os.MkdirAll(w.tmpDir, 0755)

	pcmFile := filepath.Join(w.tmpDir, fmt.Sprintf("stt_%d.pcm", len(audioData)))
	if err := os.WriteFile(pcmFile, audioData, 0644); err != nil {
		return "", fmt.Errorf("write pcm: %w", err)
	}
	defer os.Remove(pcmFile)

	wavFile := filepath.Join(w.tmpDir, fmt.Sprintf("stt_%d.wav", len(audioData)))
	cmd := exec.Command("ffmpeg",
		"-f", "s16le", "-ar", "16000", "-ac", "1", "-i", pcmFile,
		"-f", "wav", wavFile,
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("pcm→wav conversion: %w (output: %s)", err, string(out))
	}
	defer os.Remove(wavFile)

	whisperCmd := exec.Command("faster-whisper",
		"--model", w.model,
		"--language", w.language,
		"--output_format", "txt",
		wavFile,
	)
	var stdout bytes.Buffer
	whisperCmd.Stdout = &stdout
	if err := whisperCmd.Run(); err != nil {
		return "", fmt.Errorf("whisper transcription: %w", err)
	}

	return strings.TrimSpace(stdout.String()), nil
}
