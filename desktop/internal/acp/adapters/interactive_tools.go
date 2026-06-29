package adapters

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"sync"
)

// NoInteractiveQuestionToolPrompt is appended to the system prompt for agents
// that cannot be hard-configured to disable their built-in question tools. It
// instructs the model to ask clarifying questions in plain text instead of
// invoking a structured question tool, because the device-side ACP client
// cannot answer those tools over the standard protocol.
const NoInteractiveQuestionToolPrompt = " Do not use any ask-user, question, or interactive choice tool. If you need clarification, ask it in plain text."

// claudeWrapperOnce ensures we create the claude wrapper script at most once
// per process, even if multiple ClaudeAdapter instances are constructed.
var claudeWrapperOnce sync.Once

// claudeWrapperPath holds the path to the generated wrapper after the first
// successful creation. It remains empty if wrapper creation failed.
var claudeWrapperPath string

// ensureClaudeWrapper creates a tiny shell wrapper around the real `claude`
// binary that injects `--disallowedTools AskUserQuestion`. The wrapper is
// placed under ~/.elf/bin so the embedded acp-adapter runtime can use it as
// cfg.ClaudeBin.
func ensureClaudeWrapper() string {
	claudeWrapperOnce.Do(func() {
		realBin := os.Getenv("CLAUDE_BIN")
		if realBin == "" {
			if p, err := exec.LookPath("claude"); err == nil {
				realBin = p
			}
		}
		if realBin == "" {
			return
		}

		home, err := os.UserHomeDir()
		if err != nil {
			return
		}
		binDir := filepath.Join(home, ".elf", "bin")
		if err := os.MkdirAll(binDir, 0o755); err != nil {
			return
		}

		wrapper := filepath.Join(binDir, "claude-wrapper")
		script := fmt.Sprintf("#!/bin/sh\nexec %s --disallowedTools AskUserQuestion \"$@\"\n", strconv.Quote(realBin))
		if err := os.WriteFile(wrapper, []byte(script), 0o755); err != nil {
			return
		}
		claudeWrapperPath = wrapper
	})
	return claudeWrapperPath
}
