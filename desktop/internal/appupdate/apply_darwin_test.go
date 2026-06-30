//go:build darwin

package appupdate

import (
	"os"
	"path/filepath"
	"testing"
)

func TestBundlePathFromExe(t *testing.T) {
	exe := "/Applications/Anya.app/Contents/MacOS/anya"
	got, err := bundlePathFromExe(exe)
	if err != nil {
		t.Fatalf("bundlePathFromExe: %v", err)
	}
	if got != "/Applications/Anya.app" {
		t.Fatalf("got %q", got)
	}
	if _, err := bundlePathFromExe("/usr/local/bin/anya"); err == nil {
		t.Fatal("expected error for non-bundle path")
	}
}

func TestSwapDirAtomic(t *testing.T) {
	root := t.TempDir()
	cur := filepath.Join(root, "Anya.app")
	os.MkdirAll(filepath.Join(cur, "Contents"), 0o755)
	os.WriteFile(filepath.Join(cur, "Contents", "old"), []byte("old"), 0o644)

	next := filepath.Join(root, "new", "Anya.app")
	os.MkdirAll(filepath.Join(next, "Contents"), 0o755)
	os.WriteFile(filepath.Join(next, "Contents", "new"), []byte("new"), 0o644)

	if err := swapDir(next, cur); err != nil {
		t.Fatalf("swapDir: %v", err)
	}
	if _, err := os.Stat(filepath.Join(cur, "Contents", "new")); err != nil {
		t.Fatalf("new content not in place: %v", err)
	}
	if _, err := os.Stat(filepath.Join(cur, "Contents", "old")); err == nil {
		t.Fatal("old content should be gone")
	}
}
