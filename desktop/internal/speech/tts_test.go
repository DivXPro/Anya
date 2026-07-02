package speech

import (
	"encoding/binary"
	"io"
	"os"
	"testing"

	"github.com/hajimehoshi/go-mp3"
)

func TestMP3ToPCM16kHzS16LE(t *testing.T) {
	f, err := os.Open("testdata/sample_24khz.mp3")
	if err != nil {
		t.Fatalf("open fixture: %v", err)
	}
	defer f.Close()

	// Decode once to know the original sample count and rate.
	dec, err := mp3.NewDecoder(f)
	if err != nil {
		t.Fatalf("mp3 decoder init: %v", err)
	}
	pcmBytes, err := io.ReadAll(dec)
	if err != nil {
		t.Fatalf("mp3 decode: %v", err)
	}
	if dec.SampleRate() != 24000 {
		t.Fatalf("fixture sample rate should be 24000 Hz, got %d", dec.SampleRate())
	}
	inputFrames := len(pcmBytes) / (mp3DecoderChannels * 2)

	// Re-open and run the function under test.
	if _, err := f.Seek(0, 0); err != nil {
		t.Fatalf("seek fixture: %v", err)
	}

	out, err := mp3ToPCM16kHzS16LE(f)
	if err != nil {
		t.Fatalf("convert: %v", err)
	}

	if len(out) == 0 {
		t.Fatal("expected non-empty pcm output")
	}
	if len(out)%2 != 0 {
		t.Fatalf("pcm output length must be even, got %d", len(out))
	}

	outputSamples := len(out) / 2
	expectedRatio := float64(ttsOutputSampleRate) / float64(dec.SampleRate())
	actualRatio := float64(outputSamples) / float64(inputFrames)
	tolerance := 0.05

	if actualRatio < expectedRatio-tolerance || actualRatio > expectedRatio+tolerance {
		t.Fatalf("resample ratio mismatch: expected %.4f±%.2f, got %.4f (input_frames=%d output_samples=%d)",
			expectedRatio, tolerance, actualRatio, inputFrames, outputSamples)
	}

	inputDuration := float64(inputFrames) / float64(dec.SampleRate())
	outputDuration := float64(outputSamples) / float64(ttsOutputSampleRate)
	if diff := inputDuration - outputDuration; diff < -0.05 || diff > 0.05 {
		t.Fatalf("duration changed after conversion: input=%.3fs output=%.3fs", inputDuration, outputDuration)
	}

	// Spot-check that bytes are valid little-endian int16 (non-trivial energy).
	var energy int64
	for i := 0; i < len(out); i += 2 {
		s := int16(binary.LittleEndian.Uint16(out[i:]))
		energy += int64(s) * int64(s)
	}
	if energy == 0 {
		t.Fatal("output audio is silent")
	}
}
