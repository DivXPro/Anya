package speech

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestWritePCMToWav(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "out.wav")
	pcm := []byte{0x00, 0x00, 0x01, 0x00} // two mono s16le samples
	if err := writePCMToWav(pcm, 16000, out); err != nil {
		t.Fatalf("writePCMToWav failed: %v", err)
	}
	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("read wav failed: %v", err)
	}
	if len(data) != 44+len(pcm) {
		t.Fatalf("wav length mismatch: got %d, want %d", len(data), 44+len(pcm))
	}
	if string(data[0:4]) != "RIFF" || string(data[8:12]) != "WAVE" {
		t.Fatalf("invalid wav header: %s", string(data[0:12]))
	}
}

func TestWhisperCppSTT_Integration(t *testing.T) {
	cli, err := exec.LookPath("whisper-cli")
	if err != nil {
		t.Skip("whisper-cli not found in PATH")
	}
	model := os.Getenv("WHISPER_MODEL")
	if model == "" {
		// Try a few common locations.
		candidates := []string{
			filepath.Join(os.Getenv("HOME"), ".elf", "models", "ggml-tiny.bin"),
			"/opt/homebrew/Cellar/whisper-cpp/1.8.4/share/whisper-cpp/ggml-tiny.bin",
		}
		for _, c := range candidates {
			if _, err := os.Stat(c); err == nil {
				model = c
				break
			}
		}
	}
	if model == "" {
		t.Skip("WHISPER_MODEL not set and no default model found")
	}

	engine := NewWhisperCppSTT(model, "en", cli)
	// English "hello" at 16 kHz mono s16le is roughly this PCM signature;
	// the test mainly verifies the pipeline runs without errors.
	text, err := engine.Transcribe([]byte{0x00, 0x00, 0x00, 0x00}, "pcm")
	if err != nil {
		t.Fatalf("transcribe failed: %v", err)
	}
	_ = text
}
