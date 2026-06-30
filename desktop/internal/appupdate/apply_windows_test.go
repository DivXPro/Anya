//go:build windows

package appupdate

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReplaceExe(t *testing.T) {
	dir := t.TempDir()
	cur := filepath.Join(dir, "anya.exe")
	os.WriteFile(cur, []byte("old"), 0o644)
	newExe := filepath.Join(dir, "new", "anya.exe")
	os.MkdirAll(filepath.Dir(newExe), 0o755)
	os.WriteFile(newExe, []byte("new"), 0o644)

	if err := replaceExe(newExe, cur); err != nil {
		t.Fatalf("replaceExe: %v", err)
	}
	got, _ := os.ReadFile(cur)
	if string(got) != "new" {
		t.Fatalf("cur content=%q want new", string(got))
	}
}
