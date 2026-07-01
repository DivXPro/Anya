package agentinstall

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestDetectInstallMethod_NodeModulesIsNpmFamily(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink-based fixture is unix-specific")
	}
	tmp := t.TempDir()

	// A package-manager install: the real binary lives under node_modules and a
	// symlink in a bin dir points at it.
	nm := filepath.Join(tmp, "lib", "node_modules", "@anthropic-ai", "claude-code", "cli.js")
	if err := os.MkdirAll(filepath.Dir(nm), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(nm, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	binDir := filepath.Join(tmp, "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(binDir, "claude")
	if err := os.Symlink(nm, link); err != nil {
		t.Fatal(err)
	}
	if m := detectInstallMethod(link); m != MethodNpm {
		t.Fatalf("expected MethodNpm for a node_modules install, got %v", m)
	}
}

func TestDetectInstallMethod_StandaloneIsScript(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fixture is unix-specific")
	}
	tmp := t.TempDir()
	standalone := filepath.Join(tmp, "kimi")
	if err := os.WriteFile(standalone, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if m := detectInstallMethod(standalone); m != MethodScript {
		t.Fatalf("expected MethodScript for a standalone binary, got %v", m)
	}
}

func TestBuildAgentCommand_FreshInstallPrefersScript(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("script path is unix-specific")
	}
	args, display, err := buildAgentCommand("claude-code", false)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if !strings.Contains(display, "curl") {
		t.Fatalf("expected a curl-based installer, got display %q", display)
	}
	if len(args) != 3 || filepath.Base(args[0]) != "bash" || args[1] != "-c" {
		t.Fatalf("expected [bash -c <script>], got %v", args)
	}
	body := args[2]
	// Robustness invariants: download to a temp file (not a curl|bash pipe) and
	// fall back to a plain npm global install.
	if !strings.Contains(body, "mktemp") {
		t.Fatalf("expected download-to-temp, got body:\n%s", body)
	}
	if !strings.Contains(body, "install -g '@anthropic-ai/claude-code@latest'") {
		t.Fatalf("expected npm fallback, got body:\n%s", body)
	}
}

func TestBuildPMCommand_UsesLatestSpec(t *testing.T) {
	info := Registry["claude-code"]
	cases := []struct {
		pm      string
		wantSub string
	}{
		{"npm", "install -g @anthropic-ai/claude-code@latest"},
		{"pnpm", "add -g @anthropic-ai/claude-code@latest"},
		{"yarn", "global add @anthropic-ai/claude-code@latest"},
	}
	for _, tc := range cases {
		args, display, err := buildPMCommand(info, tc.pm)
		if err != nil {
			t.Fatalf("%s: %v", tc.pm, err)
		}
		if !strings.Contains(display, tc.wantSub) {
			t.Fatalf("%s: display = %q, want substring %q", tc.pm, display, tc.wantSub)
		}
		if args[len(args)-1] != "@anthropic-ai/claude-code@latest" {
			t.Fatalf("%s: expected @latest spec as last arg, got %v", tc.pm, args)
		}
	}
}
