package speech

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/ardanlabs/bucky/pkg/download"
)

// DefaultWhisperVersion is the whisper.cpp release tag that bucky is tested against.
const DefaultWhisperVersion = "v1.9.1"

// DefaultModelName is the default Whisper model downloaded on first run.
const DefaultModelName = "ggml-small.bin"

// EnsureAssets checks for the whisper.cpp shared library and the model file,
// downloading either one if missing. It returns the resolved library directory
// and model path.
func EnsureAssets(dataDir string) (libDir, modelPath string, err error) {
	libDir = filepath.Join(dataDir, "lib")
	modelDir := filepath.Join(dataDir, "models")
	modelPath = filepath.Join(modelDir, DefaultModelName)

	if err := os.MkdirAll(libDir, 0755); err != nil {
		return "", "", fmt.Errorf("create lib dir: %w", err)
	}
	if err := os.MkdirAll(modelDir, 0755); err != nil {
		return "", "", fmt.Errorf("create model dir: %w", err)
	}

	if !download.AlreadyInstalled(libDir) {
		processor := defaultProcessor()
		if err := download.Get(runtime.GOARCH, runtime.GOOS, processor, DefaultWhisperVersion, libDir); err != nil {
			return "", "", fmt.Errorf("download whisper.cpp library (%s/%s/%s): %w", runtime.GOOS, runtime.GOARCH, processor, err)
		}
	}

	if _, err := os.Stat(modelPath); os.IsNotExist(err) {
		url := fmt.Sprintf("https://huggingface.co/ggerganov/whisper.cpp/resolve/main/%s", DefaultModelName)
		if err := download.GetModel(url, modelPath); err != nil {
			return "", "", fmt.Errorf("download model %s: %w", DefaultModelName, err)
		}
	}

	return libDir, modelPath, nil
}

func defaultProcessor() string {
	if runtime.GOOS == "darwin" {
		return "metal"
	}
	return "cpu"
}
