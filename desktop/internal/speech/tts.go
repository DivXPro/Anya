package speech

import (
	"fmt"
	"io"
	"os"
	"os/exec"
)

type TTSEngine interface {
	Synthesize(text string) (<-chan []byte, error)
}

type EdgeTTS struct {
	voice string
	speed string
}

func NewEdgeTTS(voice, speed string) *EdgeTTS {
	return &EdgeTTS{voice: voice, speed: speed}
}

func (e *EdgeTTS) Synthesize(text string) (<-chan []byte, error) {
	tts := exec.Command("edge-tts",
		"--voice", e.voice,
		"--rate", e.speed,
		"--text", text,
		"--write-media", "-",
	)

	ffmpeg := exec.Command("ffmpeg",
		"-i", "pipe:0",
		"-f", "s16le",
		"-ar", "16000",
		"-ac", "1",
		"-",
	)

	ttsOut, err := tts.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("tts stdout pipe: %w", err)
	}
	ffmpeg.Stdin = ttsOut
	ffmpegOut, err := ffmpeg.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("ffmpeg stdout pipe: %w", err)
	}
	ffmpeg.Stderr = os.Stderr

	if err := ffmpeg.Start(); err != nil {
		return nil, fmt.Errorf("ffmpeg start: %w", err)
	}
	if err := tts.Start(); err != nil {
		ffmpeg.Process.Kill()
		return nil, fmt.Errorf("tts start: %w", err)
	}

	ch := make(chan []byte, 64)
	go func() {
		defer close(ch)
		defer tts.Wait()
		defer ffmpeg.Wait()

		buf := make([]byte, 4096)
		for {
			n, err := ffmpegOut.Read(buf)
			if n > 0 {
				chunk := make([]byte, n)
				copy(chunk, buf[:n])
				ch <- chunk
			}
			if err == io.EOF {
				break
			}
			if err != nil {
				return
			}
		}
	}()

	return ch, nil
}
