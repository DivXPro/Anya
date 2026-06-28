package agentinstall

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// AgentInfo holds the metadata required to detect and install an agent.
type AgentInfo struct {
	ID       string
	Name     string
	Binary   string // executable name, e.g. "claude"
	Command  string // full command used by Anya, e.g. "claude --acp"
	Packages map[string]string
	// Scripts maps GOOS to an official one-line installer for that platform.
	// When present and the required shell is available, it takes precedence
	// over package-manager installs.
	Scripts map[string]PlatformScript
}

// PlatformScript describes a platform-specific one-line installer.
type PlatformScript struct {
	Shell   string // "powershell", "bash", or "sh"
	Command string // the script body, e.g. "irm https://example.com/install.ps1 | iex"
}

// Registry maps agent IDs to their install metadata.
var Registry = map[string]AgentInfo{
	"claude-code": {
		ID:      "claude-code",
		Name:    "Claude Code",
		Binary:  "claude",
		Command: "claude --acp",
		Packages: map[string]string{
			"npm":  "@anthropic-ai/claude-code",
			"pnpm": "@anthropic-ai/claude-code",
			"yarn": "@anthropic-ai/claude-code",
		},
		Scripts: map[string]PlatformScript{
			"windows": {Shell: "powershell", Command: "irm https://claude.ai/install.ps1 | iex"},
			"darwin":  {Shell: "bash", Command: "curl -fsSL https://claude.ai/install.sh | bash"},
			"linux":   {Shell: "bash", Command: "curl -fsSL https://claude.ai/install.sh | bash"},
		},
	},
	"opencode": {
		ID:      "opencode",
		Name:    "OpenCode",
		Binary:  "opencode",
		Command: "opencode acp",
		Packages: map[string]string{
			"npm":  "opencode-ai",
			"pnpm": "opencode-ai",
			"yarn": "opencode-ai",
		},
		Scripts: map[string]PlatformScript{
			"darwin": {Shell: "bash", Command: "curl -fsSL https://opencode.ai/install | bash"},
			"linux":  {Shell: "bash", Command: "curl -fsSL https://opencode.ai/install | bash"},
		},
	},
	"codex": {
		ID:      "codex",
		Name:    "Codex",
		Binary:  "codex",
		Command: "codex app-server --stdio",
		Packages: map[string]string{
			"npm":  "@openai/codex",
			"pnpm": "@openai/codex",
			"yarn": "@openai/codex",
		},
		Scripts: map[string]PlatformScript{
			"windows": {Shell: "powershell", Command: "irm https://chatgpt.com/codex/install.ps1 | iex"},
			"darwin":  {Shell: "sh", Command: "curl -fsSL https://chatgpt.com/codex/install.sh | sh"},
			"linux":   {Shell: "sh", Command: "curl -fsSL https://chatgpt.com/codex/install.sh | sh"},
		},
	},
	"kimi": {
		ID:      "kimi",
		Name:    "Kimi Code",
		Binary:  "kimi",
		Command: "kimi acp",
		Packages: map[string]string{
			"npm":  "@moonshot-ai/kimi-code",
			"pnpm": "@moonshot-ai/kimi-code",
			"yarn": "@moonshot-ai/kimi-code",
		},
		Scripts: map[string]PlatformScript{
			"windows": {Shell: "powershell", Command: "irm https://code.kimi.com/kimi-code/install.ps1 | iex"},
			"darwin":  {Shell: "bash", Command: "curl -fsSL https://code.kimi.com/kimi-code/install.sh | bash"},
			"linux":   {Shell: "bash", Command: "curl -fsSL https://code.kimi.com/kimi-code/install.sh | bash"},
		},
	},
	"hermes": {
		ID:      "hermes",
		Name:    "Hermes",
		Binary:  "hermes",
		Command: "hermes acp",
		Packages: map[string]string{
			"npm":  "hermes-agent",
			"pnpm": "hermes-agent",
			"yarn": "hermes-agent",
		},
		Scripts: map[string]PlatformScript{
			"windows": {Shell: "powershell", Command: "iex (irm https://hermes-agent.nousresearch.com/install.ps1)"},
			"darwin":  {Shell: "bash", Command: "curl -fsSL https://hermes-agent.nousresearch.com/install.sh | bash"},
			"linux":   {Shell: "bash", Command: "curl -fsSL https://hermes-agent.nousresearch.com/install.sh | bash"},
		},
	},
	"pi": {
		ID:      "pi",
		Name:    "Pi",
		Binary:  "pi",
		Command: "pi --mode rpc --no-session",
		Packages: map[string]string{
			"npm":  "@earendil-works/pi-coding-agent",
			"pnpm": "@earendil-works/pi-coding-agent",
			"yarn": "@earendil-works/pi-coding-agent",
		},
		Scripts: map[string]PlatformScript{
			"darwin": {Shell: "sh", Command: "curl -fsSL https://pi.dev/install.sh | sh"},
			"linux":  {Shell: "sh", Command: "curl -fsSL https://pi.dev/install.sh | sh"},
		},
	},
}

// DetectPackageManager returns the first available npm-compatible package manager.
// Priority: npm > pnpm > yarn.
func DetectPackageManager() (string, string) {
	for _, name := range []string{"npm", "pnpm", "yarn"} {
		if path, err := exec.LookPath(name); err == nil {
			return name, path
		}
	}
	return "", ""
}

// InstallCommand builds the install command for the given agent and package manager.
func InstallCommand(agentID, pm string) (string, error) {
	info, ok := Registry[agentID]
	if !ok {
		return "", fmt.Errorf("unknown agent %q", agentID)
	}
	pkg, ok := info.Packages[pm]
	if !ok {
		return "", fmt.Errorf("no package defined for %s/%s", agentID, pm)
	}
	switch pm {
	case "npm":
		return fmt.Sprintf("npm install -g %s", pkg), nil
	case "pnpm":
		return fmt.Sprintf("pnpm add -g %s", pkg), nil
	case "yarn":
		return fmt.Sprintf("yarn global add %s", pkg), nil
	default:
		return "", fmt.Errorf("unsupported package manager %q", pm)
	}
}

// PlatformInstallCommand returns the best install command for the current OS.
// It prefers official one-line installers, then falls back to npm/pnpm/yarn.
// The returned slice is the exec.Command arguments, and the string is a
// human-readable form suitable for display.
func PlatformInstallCommand(agentID string) ([]string, string, error) {
	info, ok := Registry[agentID]
	if !ok {
		return nil, "", fmt.Errorf("unknown agent %q", agentID)
	}

	goos := runtime.GOOS
	if script, ok := info.Scripts[goos]; ok {
		if args, display, err := buildScriptCommand(script); err == nil {
			return args, display, nil
		}
	}

	pm, _ := DetectPackageManager()
	if pm == "" {
		return nil, "", fmt.Errorf("no package manager found")
	}
	cmd, err := InstallCommand(agentID, pm)
	if err != nil {
		return nil, "", err
	}
	return strings.Fields(cmd), cmd, nil
}

func buildScriptCommand(s PlatformScript) ([]string, string, error) {
	switch s.Shell {
	case "powershell":
		exe, err := exec.LookPath("powershell.exe")
		if err != nil {
			return nil, "", err
		}
		return []string{exe, "-Command", s.Command}, s.Command, nil
	case "bash":
		exe, err := exec.LookPath("bash")
		if err != nil {
			return nil, "", err
		}
		return []string{exe, "-c", s.Command}, s.Command, nil
	case "sh":
		exe, err := exec.LookPath("sh")
		if err != nil {
			return nil, "", err
		}
		return []string{exe, "-c", s.Command}, s.Command, nil
	default:
		return nil, "", fmt.Errorf("unsupported shell %q", s.Shell)
	}
}

// GlobalBinDir returns the global bin directory for a package manager.
// If it cannot be determined, an empty string is returned.
func GlobalBinDir(pm string) string {
	switch pm {
	case "npm":
		out, err := exec.Command("npm", "prefix", "-g").Output()
		if err != nil {
			return ""
		}
		prefix := strings.TrimSpace(string(out))
		if runtime.GOOS == "windows" {
			// npm global prefix on Windows already contains the bin dir
			return prefix
		}
		return filepath.Join(prefix, "bin")
	case "pnpm":
		out, err := exec.Command("pnpm", "bin", "-g").Output()
		if err != nil {
			return ""
		}
		return strings.TrimSpace(string(out))
	case "yarn":
		out, err := exec.Command("yarn", "global", "bin").Output()
		if err != nil {
			return ""
		}
		return strings.TrimSpace(string(out))
	}
	return ""
}

// Platform returns the runtime platform string used for logging/UI.
func Platform() string {
	return fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH)
}
