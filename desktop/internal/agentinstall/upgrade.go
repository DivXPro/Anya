package agentinstall

import (
	"fmt"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
)

// UpgradeMethod describes how an agent CLI is (or should be) installed, so an
// update can be performed IN-PLACE via the same mechanism instead of blindly
// re-running the vendor installer. This "anchoring" avoids creating a second,
// parallel install and sidesteps GUI-PATH ambiguity.
type UpgradeMethod int

const (
	MethodUnknown UpgradeMethod = iota
	MethodScript                // vendor one-line installer (curl … | bash, irm … | iex)
	MethodNpm
	MethodPnpm
	MethodYarn
)

// scriptURLRe extracts the download URL from a vendor one-line installer so we
// can fetch it to a temp file and execute that, instead of piping curl into a
// shell (a pipe can hang half-executed when a proxy stalls mid-stream).
var scriptURLRe = regexp.MustCompile(`https?://[^\s'"|]+`)

// pmForMethod maps an npm-family method to its package-manager command name.
func pmForMethod(m UpgradeMethod) string {
	switch m {
	case MethodPnpm:
		return "pnpm"
	case MethodYarn:
		return "yarn"
	case MethodNpm:
		return "npm"
	default:
		return ""
	}
}

// sameDir reports whether two directory paths refer to the same location.
func sameDir(a, b string) bool {
	if a == "" || b == "" {
		return false
	}
	return filepath.Clean(a) == filepath.Clean(b)
}

// detectInstallMethod infers how the CLI at binaryPath was installed by
// resolving symlinks and inspecting the real target: package-manager global
// installs land under node_modules, while vendor installers drop a standalone
// binary elsewhere. When node_modules is involved, the owning package manager
// is identified by matching the symlink's directory against each global bin dir.
func detectInstallMethod(binaryPath string) UpgradeMethod {
	if binaryPath == "" {
		return MethodUnknown
	}
	resolved := binaryPath
	if r, err := filepath.EvalSymlinks(binaryPath); err == nil {
		resolved = r
	}
	lower := strings.ToLower(filepath.ToSlash(resolved))
	sep := "/"
	if strings.Contains(lower, sep+"node_modules"+sep) || strings.HasSuffix(lower, sep+"node_modules") {
		dir := filepath.Dir(binaryPath)
		if bin := GlobalBinDir("pnpm"); sameDir(dir, bin) {
			return MethodPnpm
		}
		if bin := GlobalBinDir("yarn"); sameDir(dir, bin) {
			return MethodYarn
		}
		return MethodNpm
	}
	return MethodScript
}

// resolveMethod decides which install method to use. For an update it anchors
// to how the existing binary was installed; for a fresh install it prefers the
// vendor script when one exists for this OS, otherwise an npm-family install.
func resolveMethod(info AgentInfo, isUpdate bool) UpgradeMethod {
	if isUpdate {
		if p := findBinary(info.Binary); p != "" {
			if m := detectInstallMethod(p); m != MethodUnknown {
				return m
			}
		}
	}
	if _, ok := info.Scripts[runtime.GOOS]; ok {
		return MethodScript
	}
	return MethodNpm
}

// buildAgentCommand builds the exec args and a human-readable display string for
// installing or upgrading an agent, anchored to how it is already installed.
//
//   - npm/pnpm/yarn: run "<pm> install -g <pkg>@latest" using an absolute path
//     to the package manager when it can be located.
//   - vendor script (Unix): download the installer to a temp file and run it
//     with a fallback to a plain npm global install if the download or the
//     script fails.
//   - vendor script (Windows): run the official PowerShell one-liner.
func buildAgentCommand(id string, isUpdate bool) ([]string, string, error) {
	info, ok := Registry[id]
	if !ok {
		return nil, "", fmt.Errorf("unknown agent %q", id)
	}

	method := resolveMethod(info, isUpdate)

	// npm-family install/upgrade.
	if pm := pmForMethod(method); pm != "" {
		return buildPMCommand(info, pm)
	}

	// Vendor script method.
	script, ok := info.Scripts[runtime.GOOS]
	if !ok {
		// No script for this OS; fall back to npm-family if possible.
		return buildPMCommand(info, "")
	}

	if runtime.GOOS == "windows" {
		return buildWindowsScriptCommand(info, script)
	}
	return buildUnixScriptCommand(info, script)
}

// buildPMCommand builds a package-manager global install command for pkg@latest.
// When pm is empty the first available package manager is auto-detected.
func buildPMCommand(info AgentInfo, pm string) ([]string, string, error) {
	if pm == "" {
		pm, _ = DetectPackageManager()
	}
	if pm == "" {
		return nil, "", fmt.Errorf("no package manager found")
	}
	pkg := info.Packages[pm]
	if pkg == "" {
		return nil, "", fmt.Errorf("no %s package defined for %s", pm, info.ID)
	}
	spec := pkg + "@latest"

	// Prefer an absolute path so the command does not depend on the (possibly
	// minimal) GUI PATH resolving the package manager.
	exe := findBinary(pm)
	if exe == "" {
		exe = pm
	}

	var verb []string
	switch pm {
	case "npm":
		verb = []string{"install", "-g", spec}
	case "pnpm":
		verb = []string{"add", "-g", spec}
	case "yarn":
		verb = []string{"global", "add", spec}
	default:
		return nil, "", fmt.Errorf("unsupported package manager %q", pm)
	}

	args := append([]string{exe}, verb...)
	display := pm + " " + strings.Join(verb, " ")
	return args, display, nil
}

// buildUnixScriptCommand generates a bash script that downloads the vendor
// installer to a temp file and runs it, falling back to an npm global install
// when the download or the script fails. Downloading before executing avoids
// the half-executed hang that a `curl … | bash` pipe suffers when a proxy
// stalls mid-transfer.
func buildUnixScriptCommand(info AgentInfo, script PlatformScript) ([]string, string, error) {
	url := scriptURLRe.FindString(script.Command)
	if url == "" {
		// Can't rewrite it safely; run the original one-liner as-is.
		return buildScriptCommand(script)
	}

	runShell := "bash"
	if script.Shell == "sh" {
		runShell = "sh"
	}

	// Optional npm fallback (only when the agent has an npm package).
	fallback := ""
	fallbackDisplay := ""
	if pkg := info.Packages["npm"]; pkg != "" {
		npm := findBinary("npm")
		if npm == "" {
			npm = "npm"
		}
		fallback = fmt.Sprintf("echo '[anya] official installer failed; falling back to npm' >&2; exec %s install -g %s",
			shQuote(npm), shQuote(pkg+"@latest"))
		fallbackDisplay = " || npm install -g " + pkg + "@latest"
	}

	body := strings.Join([]string{
		"set -u",
		"URL=" + shQuote(url),
		"run_official() {",
		"  tmp=\"$(mktemp)\" || return 1",
		"  if curl -fsSL \"$URL\" -o \"$tmp\"; then",
		"    " + runShell + " \"$tmp\"; rc=$?",
		"  else",
		"    rc=1",
		"  fi",
		"  rm -f \"$tmp\"",
		"  return $rc",
		"}",
		"run_official" + fallbackChain(fallback),
	}, "\n")

	bash := findBinary("bash")
	if bash == "" {
		bash = "/bin/bash"
	}
	display := fmt.Sprintf("curl -fsSL %s | %s%s", url, runShell, fallbackDisplay)
	return []string{bash, "-c", body}, display, nil
}

func fallbackChain(fallback string) string {
	if fallback == "" {
		return ""
	}
	return " || { " + fallback + "; }"
}

// buildWindowsScriptCommand runs the official PowerShell installer. When the
// agent has an npm package, an npm global install is appended as a fallback.
func buildWindowsScriptCommand(info AgentInfo, script PlatformScript) ([]string, string, error) {
	args, display, err := buildScriptCommand(script)
	if err != nil {
		// PowerShell unavailable; try npm instead.
		return buildPMCommand(info, "")
	}
	if pkg := info.Packages["npm"]; pkg != "" && len(args) == 3 {
		// args == [powershell, -Command, <cmd>]; append npm fallback.
		args[2] = fmt.Sprintf("try { %s } catch { npm install -g %s@latest }", script.Command, pkg)
		display += " (fallback: npm install -g " + pkg + "@latest)"
	}
	return args, display, nil
}

// shQuote single-quotes a string for safe embedding in a POSIX shell command.
func shQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
