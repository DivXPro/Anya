package speech

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"log"

	edgetts "github.com/foresturquhart/edge-tts"
	"github.com/hajimehoshi/go-mp3"
	"github.com/tphakala/go-audio-resampler"
)

const (
	ttsOutputSampleRate = 16000
	ttsOutputChannels   = 1
)

// TTSEngine turns text into a stream of PCM audio chunks.
type TTSEngine interface {
	Synthesize(text string) (<-chan []byte, error)
}

// EdgeTTS uses Microsoft Edge's online text-to-speech service. The synthesized
// MP3 stream is decoded and resampled to 16 kHz s16le mono PCM entirely in Go,
// avoiding external ffmpeg/edge-tts CLI dependencies.
type EdgeTTS struct {
	voice string
	rate  string
}

func NewEdgeTTS(voice, rate string) *EdgeTTS {
	return &EdgeTTS{voice: voice, rate: rate}
}

func (e *EdgeTTS) Synthesize(text string) (<-chan []byte, error) {
	cfg := edgetts.DefaultConfig()
	cfg.Voice = e.voice
	if e.rate != "" {
		cfg.Rate = e.rate
	}

	comm, err := edgetts.NewCommunicate(text, cfg)
	if err != nil {
		return nil, fmt.Errorf("edge-tts init: %w", err)
	}

	ch := make(chan []byte, 64)
	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		defer close(ch)
		defer cancel()

		if err := e.synthesize(ctx, comm, ch); err != nil {
			log.Printf("[tts] synthesize error: %v", err)
		}
	}()

	return ch, nil
}

func (e *EdgeTTS) synthesize(ctx context.Context, comm *edgetts.Communicate, ch chan<- []byte) error {
	// 1. Collect the streamed MP3 audio from edge-tts.
	var mp3Buf bytes.Buffer
	if err := comm.Stream(ctx, func(chunk edgetts.TTSChunk) error {
		if chunk.Type != edgetts.ChunkTypeAudio {
			return nil
		}
		_, err := mp3Buf.Write(chunk.Data)
		return err
	}); err != nil {
		return fmt.Errorf("edge-tts stream: %w", err)
	}

	if mp3Buf.Len() == 0 {
		return fmt.Errorf("no audio received from edge-tts")
	}

	// 2. Decode MP3 and resample to 16 kHz s16le mono PCM.
	out, err := mp3ToPCM16kHzS16LE(&mp3Buf)
	if err != nil {
		return err
	}

	// 3. Stream in chunks.
	const chunkSize = 4096
	for offset := 0; offset < len(out); offset += chunkSize {
		end := offset + chunkSize
		if end > len(out) {
			end = len(out)
		}
		chunk := make([]byte, end-offset)
		copy(chunk, out[offset:end])
		ch <- chunk
	}

	return nil
}

// mp3ToPCM16kHzS16LE decodes MP3 audio, resamples it to 16 kHz mono, and
// returns it as signed 16-bit little-endian PCM bytes.
func mp3ToPCM16kHzS16LE(r io.Reader) ([]byte, error) {
	dec, err := mp3.NewDecoder(r)
	if err != nil {
		return nil, fmt.Errorf("mp3 decoder init: %w", err)
	}

	pcmBytes, err := io.ReadAll(dec)
	if err != nil {
		return nil, fmt.Errorf("mp3 decode: %w", err)
	}
	if len(pcmBytes)%2 != 0 {
		return nil, fmt.Errorf("decoded pcm has odd length")
	}

	inputRate := float64(dec.SampleRate())
	samples := make([]float64, len(pcmBytes)/2)
	for i := range samples {
		samples[i] = float64(int16(binary.LittleEndian.Uint16(pcmBytes[i*2:])))
	}

	resampled, err := resampler.ResampleMono(samples, inputRate, ttsOutputSampleRate, resampler.QualityMedium)
	if err != nil {
		return nil, fmt.Errorf("resample: %w", err)
	}

	out := make([]byte, len(resampled)*2)
	for i, s := range resampled {
		if s > 32767 {
			s = 32767
		}
		if s < -32768 {
			s = -32768
		}
		binary.LittleEndian.PutUint16(out[i*2:], uint16(int16(s)))
	}

	return out, nil
}
