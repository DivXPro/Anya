package speech

import (
	"encoding/binary"
	"fmt"
	"strings"
	"sync"
	"unsafe"

	"github.com/ardanlabs/bucky/pkg/utils"
	"github.com/ardanlabs/bucky/pkg/whisper"
)

// BuckySTT uses ardanlabs/bucky (pure-Go FFI bindings to whisper.cpp) for
// local speech-to-text. It loads the shared library and model once and reuses
// the same whisper Context for every transcription.
type BuckySTT struct {
	modelPath string
	language  string
	ctx       whisper.Context
	mu        sync.Mutex
}

// NewBuckySTT creates a new STT engine. It loads the whisper.cpp shared library
// from libDir and the model from modelPath. Call EnsureAssets first to obtain
// valid paths.
func NewBuckySTT(libDir, modelPath, language string) (*BuckySTT, error) {
	if err := whisper.Load(libDir); err != nil {
		return nil, fmt.Errorf("load whisper library: %w", err)
	}
	if err := whisper.Init(libDir); err != nil {
		return nil, fmt.Errorf("init whisper backends: %w", err)
	}

	cparams := whisper.ContextDefaultParams()
	ctx, err := whisper.InitFromFileWithParams(modelPath, cparams)
	if err != nil {
		return nil, fmt.Errorf("load model: %w", err)
	}

	return &BuckySTT{
		modelPath: modelPath,
		language:  language,
		ctx:       ctx,
	}, nil
}

func (b *BuckySTT) Transcribe(audioData []byte, format string) (string, error) {
	samples, err := pcmS16LEToFloat32(audioData)
	if err != nil {
		return "", err
	}
	if len(samples) == 0 {
		return "", nil
	}

	langPtr, err := utils.BytePtrFromString(b.language)
	if err != nil {
		return "", fmt.Errorf("language string: %w", err)
	}

	params := whisper.FullDefaultParams(whisper.SamplingGreedy)
	params.NoTimestamps = 1
	params.PrintProgress = 0
	params.PrintRealtime = 0
	params.PrintTimestamps = 0
	params.NThreads = 4
	params.Language = uintptr(unsafe.Pointer(langPtr))

	b.mu.Lock()
	defer b.mu.Unlock()

	if err := whisper.Full(b.ctx, params, samples); err != nil {
		return "", fmt.Errorf("whisper full: %w", err)
	}

	var sb strings.Builder
	n := whisper.FullNSegments(b.ctx)
	for i := int32(0); i < n; i++ {
		sb.WriteString(whisper.FullGetSegmentText(b.ctx, i))
	}
	return strings.TrimSpace(sb.String()), nil
}

func pcmS16LEToFloat32(pcm []byte) ([]float32, error) {
	if len(pcm)%2 != 0 {
		return nil, fmt.Errorf("pcm s16le data must have even length")
	}
	samples := make([]float32, len(pcm)/2)
	for i := range samples {
		val := int16(binary.LittleEndian.Uint16(pcm[i*2:]))
		samples[i] = float32(val) / 32768.0
	}
	return samples, nil
}
