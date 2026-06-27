//go:build integration

package speech

import (
	"context"
	"net"
	"testing"
	"time"
)

func TestEdgeTTSSynthesizeProducesPCM16kHz(t *testing.T) {
	if !hasNetwork(t) {
		t.Skip("network unavailable")
	}

	tts := NewEdgeTTS("zh-CN-XiaoxiaoNeural", "+0%")
	ch, err := tts.Synthesize("你好，世界。")
	if err != nil {
		t.Fatalf("synthesize: %v", err)
	}

	var total int
	done := make(chan struct{})
	go func() {
		defer close(done)
		for chunk := range ch {
			total += len(chunk)
		}
	}()

	select {
	case <-done:
	case <-time.After(30 * time.Second):
		t.Fatal("timeout waiting for synthesized audio")
	}

	if total == 0 {
		t.Fatal("expected non-empty pcm output")
	}
	if total%2 != 0 {
		t.Fatalf("pcm output length must be even, got %d", total)
	}

	// Output is 16 kHz s16le mono; a 4-character phrase should be well
	// over 0.5 seconds.
	duration := float64(total/2) / ttsOutputSampleRate
	if duration < 0.5 {
		t.Fatalf("expected at least 0.5s of audio, got %.2fs", duration)
	}
}

func hasNetwork(t *testing.T) bool {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	var d net.Dialer
	conn, err := d.DialContext(ctx, "tcp", "www.microsoft.com:443")
	if err != nil {
		return false
	}
	conn.Close()
	return true
}
