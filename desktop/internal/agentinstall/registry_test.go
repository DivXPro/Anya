package agentinstall

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestRegistryContainsExpectedAgents(t *testing.T) {
	want := []string{"claude-code", "opencode", "codex", "kimi", "hermes", "pi"}
	for _, id := range want {
		info, ok := Registry[id]
		if !ok {
			t.Errorf("registry missing agent %q", id)
			continue
		}
		if info.ID != id {
			t.Errorf("registry ID mismatch for %q: got %q", id, info.ID)
		}
		if info.Binary == "" {
			t.Errorf("registry agent %q has empty binary", id)
		}
		if info.Command == "" {
			t.Errorf("registry agent %q has empty command", id)
		}
	}
}

func TestInstallCommand(t *testing.T) {
	cases := []struct {
		agentID string
		pm      string
		want    string
	}{
		{"claude-code", "npm", "npm install -g @anthropic-ai/claude-code"},
		{"claude-code", "pnpm", "pnpm add -g @anthropic-ai/claude-code"},
		{"opencode", "yarn", "yarn global add opencode-ai"},
		{"codex", "npm", "npm install -g @openai/codex"},
	}
	for _, c := range cases {
		got, err := InstallCommand(c.agentID, c.pm)
		if err != nil {
			t.Errorf("InstallCommand(%q, %q) unexpected error: %v", c.agentID, c.pm, err)
			continue
		}
		if got != c.want {
			t.Errorf("InstallCommand(%q, %q) = %q, want %q", c.agentID, c.pm, got, c.want)
		}
	}
}

func TestInstallCommandErrors(t *testing.T) {
	if _, err := InstallCommand("unknown-agent", "npm"); err == nil {
		t.Error("expected error for unknown agent")
	}
	if _, err := InstallCommand("claude-code", "bun"); err == nil {
		t.Error("expected error for unsupported package manager")
	}
}

func TestDetectPackageManagerPriority(t *testing.T) {
	tmp := t.TempDir()

	writeFakeBinary(t, tmp, "npm", "10.0.0")
	writeFakeBinary(t, tmp, "pnpm", "9.0.0")
	writeFakeBinary(t, tmp, "yarn", "1.22.0")
	t.Setenv("PATH", tmp)

	pm, path := DetectPackageManager()
	if pm != "npm" {
		t.Fatalf("expected npm to win, got %q", pm)
	}
	if !strings.HasSuffix(path, filepath.Join(tmp, "npm"+scriptExt())) {
		t.Fatalf("unexpected path: %q", path)
	}

	if err := os.Remove(filepath.Join(tmp, "npm"+scriptExt())); err != nil {
		t.Fatalf("remove fake npm: %v", err)
	}
	pm, _ = DetectPackageManager()
	if pm != "pnpm" {
		t.Fatalf("expected pnpm after npm removed, got %q", pm)
	}

	if err := os.Remove(filepath.Join(tmp, "pnpm"+scriptExt())); err != nil {
		t.Fatalf("remove fake pnpm: %v", err)
	}
	pm, _ = DetectPackageManager()
	if pm != "yarn" {
		t.Fatalf("expected yarn after pnpm removed, got %q", pm)
	}

	if err := os.Remove(filepath.Join(tmp, "yarn"+scriptExt())); err != nil {
		t.Fatalf("remove fake yarn: %v", err)
	}
	pm, _ = DetectPackageManager()
	if pm != "" {
		t.Fatalf("expected empty when no package manager, got %q", pm)
	}
}

func TestGlobalBinDir(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix-only test")
	}
	tmp := t.TempDir()
	writeFakeNPM(t, tmp)
	t.Setenv("PATH", tmp)

	got := GlobalBinDir("npm")
	want := filepath.Join(tmp, "bin")
	if got != want {
		t.Fatalf("GlobalBinDir(npm) = %q, want %q", got, want)
	}
}

func TestGlobalBinDirNPMOnWindows(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows-only test")
	}
	tmp := t.TempDir()
	writeFakeNPM(t, tmp)
	t.Setenv("PATH", tmp)

	got := GlobalBinDir("npm")
	if got != tmp {
		t.Fatalf("GlobalBinDir(npm) on Windows = %q, want %q", got, tmp)
	}
}
