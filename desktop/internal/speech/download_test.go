package speech

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestDownloadFileResume(t *testing.T) {
	content := []byte("hello world from whisper model")
	partial := content[:10]

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rangeHeader := r.Header.Get("Range")
		if rangeHeader == "" {
			w.Write(content)
			return
		}
		var start int
		if _, err := fmt.Sscanf(rangeHeader, "bytes=%d-", &start); err != nil {
			t.Fatalf("bad range header: %s", rangeHeader)
		}
		w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, len(content)-1, len(content)))
		w.WriteHeader(http.StatusPartialContent)
		w.Write(content[start:])
	}))
	defer srv.Close()

	dir := t.TempDir()
	dest := filepath.Join(dir, "model.bin")

	// Pre-create a partial temp file.
	if err := os.WriteFile(dest+".tmp", partial, 0644); err != nil {
		t.Fatal(err)
	}

	if err := downloadFile(srv.URL, dest); err != nil {
		t.Fatalf("download failed: %v", err)
	}

	got, err := os.ReadFile(dest)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(content) {
		t.Fatalf("unexpected content: got %q, want %q", got, content)
	}

	if _, err := os.Stat(dest + ".tmp"); !os.IsNotExist(err) {
		t.Fatal("temporary download file should be removed after completion")
	}
}

func TestDownloadFileFresh(t *testing.T) {
	content := []byte("fresh download")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Range") != "" {
			t.Fatal("fresh download should not send Range header")
		}
		w.Write(content)
	}))
	defer srv.Close()

	dir := t.TempDir()
	dest := filepath.Join(dir, "model.bin")

	if err := downloadFile(srv.URL, dest); err != nil {
		t.Fatalf("download failed: %v", err)
	}

	got, err := os.ReadFile(dest)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(content) {
		t.Fatalf("unexpected content: got %q, want %q", got, content)
	}
}

