package speech

import (
	"encoding/binary"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// WhisperCppSTT uses the whisper.cpp command-line binary to transcribe audio.
// It does not depend on Python; it only requires a compiled whisper-cli binary
// and a ggml model file.
type WhisperCppSTT struct {
	modelPath string
	language  string
	cliPath   string
	tmpDir    string
}

// NewWhisperCppSTT creates a new STT engine backed by whisper.cpp.
// cliPath may be empty to use "whisper-cli" from PATH.
func NewWhisperCppSTT(modelPath, language, cliPath string) *WhisperCppSTT {
	if cliPath == "" {
		cliPath = "whisper-cli"
	}
	return &WhisperCppSTT{
		modelPath: modelPath,
		language:  language,
		cliPath:   cliPath,
		tmpDir:    filepath.Join("elf-data", "tmp"),
	}
}

func (w *WhisperCppSTT) Transcribe(audioData []byte, format string) (string, error) {
	if _, err := os.Stat(w.modelPath); err != nil {
		return "", fmt.Errorf("whisper model not found at %s: %w", w.modelPath, err)
	}

	if err := os.MkdirAll(w.tmpDir, 0755); err != nil {
		return "", fmt.Errorf("create tmp dir: %w", err)
	}

	// audioData is expected to be 16 kHz, mono, signed 16-bit little-endian PCM.
	// whisper-cli reads WAV files, so add a minimal WAV header.
	base := fmt.Sprintf("stt_%d", len(audioData))
	wavFile := filepath.Join(w.tmpDir, base+".wav")
	outPrefix := filepath.Join(w.tmpDir, base+"_out")
	outFile := outPrefix + ".txt"

	if err := writePCMToWav(audioData, 16000, wavFile); err != nil {
		return "", fmt.Errorf("pcm→wav conversion: %w", err)
	}
	defer os.Remove(wavFile)
	defer os.Remove(outFile)

	cmd := exec.Command(w.cliPath,
		"-m", w.modelPath,
		"-f", wavFile,
		"-l", w.language,
		"-nt",   // no timestamps
		"-np",   // no prints other than results
		"-otxt", // output text file
		"-of", outPrefix,
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("whisper-cli transcription: %w (output: %s)", err, string(out))
	}

	data, err := os.ReadFile(outFile)
	if err != nil {
		return "", fmt.Errorf("read whisper output: %w", err)
	}

	return strings.TrimSpace(string(data)), nil
}

func writePCMToWav(pcm []byte, sampleRate int, outPath string) error {
	f, err := os.Create(outPath)
	if err != nil {
		return err
	}
	defer f.Close()

	header := make([]byte, 44)
	copy(header[0:4], "RIFF")
	binary.LittleEndian.PutUint32(header[4:8], uint32(36+len(pcm)))
	copy(header[8:12], "WAVE")
	copy(header[12:16], "fmt ")
	binary.LittleEndian.PutUint32(header[16:20], 16) // subchunk1 size
	binary.LittleEndian.PutUint16(header[20:22], 1)  // audio format (PCM)
	binary.LittleEndian.PutUint16(header[22:24], 1)  // channels (mono)
	binary.LittleEndian.PutUint32(header[24:28], uint32(sampleRate))
	binary.LittleEndian.PutUint32(header[28:32], uint32(sampleRate*2)) // byte rate
	binary.LittleEndian.PutUint16(header[32:34], 2)                    // block align
	binary.LittleEndian.PutUint16(header[34:36], 16)                   // bits per sample
	copy(header[36:40], "data")
	binary.LittleEndian.PutUint32(header[40:44], uint32(len(pcm)))

	if _, err := f.Write(header); err != nil {
		return err
	}
	if _, err := f.Write(pcm); err != nil {
		return err
	}
	return nil
}
