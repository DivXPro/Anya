package speech

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// DownloadProgress tracks the current model download state.
type DownloadProgress struct {
	Downloading bool  `json:"downloading"`
	Total       int64 `json:"total"`
	Downloaded  int64 `json:"downloaded"`
}

var (
	progressMu sync.RWMutex
	progress   DownloadProgress
)

// GetDownloadProgress returns a snapshot of the current model download progress.
func GetDownloadProgress() DownloadProgress {
	progressMu.RLock()
	defer progressMu.RUnlock()
	return progress
}

func setProgress(p DownloadProgress) {
	progressMu.Lock()
	progress = p
	progressMu.Unlock()
}

func updateProgress(downloaded int64) {
	progressMu.Lock()
	progress.Downloaded = downloaded
	progressMu.Unlock()
}

// downloadFile downloads a file from url to dest with resume support.
// If dest already exists and the server supports range requests, the download
// continues from the existing byte offset. Otherwise it starts from the beginning.
func downloadFile(url, dest string) error {
	// Download to a temporary file next to the destination so a partial
	// download does not leave dest corrupted.
	tmpDest := dest + ".tmp"

	// Ensure the destination directory exists.
	if err := os.MkdirAll(filepath.Dir(tmpDest), 0755); err != nil {
		return fmt.Errorf("create dest dir: %w", err)
	}

	startOffset := int64(0)
	if info, err := os.Stat(tmpDest); err == nil {
		startOffset = info.Size()
		log.Printf("[speech] resuming download from byte %d", startOffset)
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	if startOffset > 0 {
		req.Header.Set("Range", fmt.Sprintf("bytes=%d-", startOffset))
	}

	client := &http.Client{Timeout: 0}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	var out *os.File
	switch resp.StatusCode {
	case http.StatusOK:
		// Server does not support resume; start from scratch.
		startOffset = 0
		setProgress(DownloadProgress{Downloading: true, Total: resp.ContentLength})
		out, err = os.Create(tmpDest)
		if err != nil {
			return fmt.Errorf("create temp file: %w", err)
		}
	case http.StatusPartialContent:
		// Server supports resume; append to existing temp file.
		total := parseTotalSize(resp, startOffset)
		setProgress(DownloadProgress{Downloading: true, Total: total, Downloaded: startOffset})
		out, err = os.OpenFile(tmpDest, os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			return fmt.Errorf("open temp file for append: %w", err)
		}
	case http.StatusRequestedRangeNotSatisfiable:
		// Existing temp file is already complete (or larger than the resource).
		setProgress(DownloadProgress{Downloading: false, Total: startOffset, Downloaded: startOffset})
		out = nil
	default:
		return fmt.Errorf("unexpected http status %d", resp.StatusCode)
	}

	if out != nil {
		defer out.Close()

		written, err := copyWithProgress(out, resp.Body, startOffset)
		if err != nil {
			return fmt.Errorf("download body: %w", err)
		}
		log.Printf("[speech] downloaded %d bytes", written)
		if err := out.Sync(); err != nil {
			return fmt.Errorf("sync temp file: %w", err)
		}
	}

	// Verify the temp file is non-empty.
	info, err := os.Stat(tmpDest)
	if err != nil {
		return fmt.Errorf("stat temp file: %w", err)
	}
	if info.Size() == 0 {
		return fmt.Errorf("downloaded file is empty")
	}

	// Move the completed temp file to the final destination.
	if err := os.Rename(tmpDest, dest); err != nil {
		return fmt.Errorf("rename temp file: %w", err)
	}

	setProgress(DownloadProgress{Downloading: false, Total: info.Size(), Downloaded: info.Size()})
	return nil
}

// parseTotalSize tries to determine the full resource size from response headers.
func parseTotalSize(resp *http.Response, startOffset int64) int64 {
	if cr := resp.Header.Get("Content-Range"); cr != "" {
		var start, end, total int64
		if _, err := fmt.Sscanf(cr, "bytes %d-%d/%d", &start, &end, &total); err == nil {
			return total
		}
	}
	if resp.ContentLength > 0 {
		return startOffset + resp.ContentLength
	}
	return 0
}

// copyWithProgress copies from src to dst and logs progress every few MB.
func copyWithProgress(dst io.Writer, src io.Reader, startOffset int64) (int64, error) {
	const reportInterval = 10 * 1024 * 1024 // 10 MB

	total := startOffset
	nextReport := ((startOffset / reportInterval) + 1) * reportInterval
	buf := make([]byte, 64*1024)
	lastLog := time.Now()

	for {
		n, err := src.Read(buf)
		if n > 0 {
			if _, werr := dst.Write(buf[:n]); werr != nil {
				return total - startOffset, werr
			}
			total += int64(n)
			updateProgress(total)
			if total >= nextReport && time.Since(lastLog) > time.Second {
				log.Printf("[speech] downloaded %.1f MB", float64(total)/(1024*1024))
				nextReport = total + reportInterval
				lastLog = time.Now()
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return total - startOffset, err
		}
	}

	return total - startOffset, nil
}
