package agentinstall

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"desktop/internal/store"

	"github.com/wailsapp/wails/v3/pkg/application"
)

const (
	EventInstallStarted  = "agent:install:started"
	EventInstallFinished = "agent:install:finished"
	EventInstallFailed   = "agent:install:failed"
)

// Installer detects and installs agent CLIs.
type Installer struct {
	app   *application.App
	db    *sql.DB
	mu    sync.Mutex
	tasks map[string]*Task
}

type Task struct {
	AgentID string
	Running bool
}

func New(app *application.App, db *sql.DB) *Installer {
	return &Installer{
		app:   app,
		db:    db,
		tasks: make(map[string]*Task),
	}
}

// DetectAll scans every registered agent and updates its installed status + version.
func (i *Installer) DetectAll() error {
	agents, err := store.ListAgents(i.db)
	if err != nil {
		return fmt.Errorf("list agents: %w", err)
	}
	for _, ag := range agents {
		if _, ok := Registry[ag.ID]; !ok {
			continue
		}
		if err := i.detectOne(ag.ID, ag.Command); err != nil {
			log.Printf("[agentinstall] detect %s error: %v", ag.ID, err)
		}
	}
	return nil
}

// Detect updates the installed status for a single agent.
func (i *Installer) Detect(id string) error {
	ag, err := store.GetAgent(i.db, id)
	if err != nil {
		return fmt.Errorf("get agent %s: %w", id, err)
	}
	return i.detectOne(ag.ID, ag.Command)
}

func (i *Installer) detectOne(id, command string) error {
	info, ok := Registry[id]
	if !ok {
		return nil
	}
	binary := info.Binary
	if command != "" {
		binary = strings.Fields(command)[0]
	}

	installed := findBinary(binary) != ""
	var version string
	if installed {
		version = commandVersion(binary)
	}

	if err := store.UpdateAgentInstalled(i.db, id, installed); err != nil {
		return fmt.Errorf("update installed: %w", err)
	}
	if version != "" {
		if err := store.UpdateAgentVersion(i.db, id, version); err != nil {
			return fmt.Errorf("update version: %w", err)
		}
	}

	pm, _ := DetectPackageManager()
	if pm != "" {
		cmd, err := InstallCommand(id, pm)
		if err == nil {
			if err := store.UpdateAgentInstallCommand(i.db, id, cmd); err != nil {
				log.Printf("[agentinstall] save install command %s error: %v", id, err)
			}
		}
	}

	return nil
}

// IsInstalling reports whether an install is currently running for the agent.
func (i *Installer) IsInstalling(id string) bool {
	i.mu.Lock()
	defer i.mu.Unlock()
	t, ok := i.tasks[id]
	return ok && t.Running
}

// Install starts an asynchronous install for the given agent.
func (i *Installer) Install(id string) error {
	info, ok := Registry[id]
	if !ok {
		return fmt.Errorf("unknown agent %q", id)
	}

	i.mu.Lock()
	if t, ok := i.tasks[id]; ok && t.Running {
		i.mu.Unlock()
		return fmt.Errorf("install already in progress for %s", id)
	}
	i.tasks[id] = &Task{AgentID: id, Running: true}
	i.mu.Unlock()

	pm, pmPath := DetectPackageManager()
	if pm == "" {
		i.finishTask(id, false)
		return fmt.Errorf("no package manager found")
	}

	cmdStr, err := InstallCommand(id, pm)
	if err != nil {
		i.finishTask(id, false)
		return err
	}

	if err := store.UpdateAgentInstallCommand(i.db, id, cmdStr); err != nil {
		log.Printf("[agentinstall] save install command %s error: %v", id, err)
	}

	i.emit(EventInstallStarted, map[string]string{"agentID": id})

	go func() {
		parts := strings.Fields(cmdStr)
		if len(parts) == 0 {
			i.emitFailure(id, "empty install command")
			return
		}

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer cancel()

		cmd := exec.CommandContext(ctx, parts[0], parts[1:]...)
		// Ensure the chosen package manager is on PATH for the child process.
		cmd.Env = append(os.Environ(), fmt.Sprintf("PATH=%s", augmentPath(pmPath)))

		log.Printf("[agentinstall] installing %s: %s", id, cmdStr)
		out, err := cmd.CombinedOutput()
		if err != nil {
			msg := string(out)
			if msg == "" {
				msg = err.Error()
			} else {
				msg = lastLines(msg, 4)
			}
			i.emitFailure(id, msg)
			return
		}

		// Give the filesystem a moment to settle, then re-detect.
		time.Sleep(500 * time.Millisecond)
		if derr := i.detectOne(id, info.Command); derr != nil {
			log.Printf("[agentinstall] post-install detect %s error: %v", id, derr)
		}

		ag, _ := store.GetAgent(i.db, id)
		version := ""
		if ag.Version != nil {
			version = *ag.Version
		}
		if !ag.Installed {
			i.emitFailure(id, "installation completed but binary was not found on PATH")
			return
		}

		i.emit(EventInstallFinished, map[string]string{"agentID": id, "version": version})
		i.finishTask(id, true)
	}()

	return nil
}

func (i *Installer) emit(name string, data any) {
	if i.app != nil && i.app.Event != nil {
		i.app.Event.Emit(name, data)
	}
}

func (i *Installer) emitFailure(id, message string) {
	log.Printf("[agentinstall] install %s failed: %s", id, message)
	i.emit(EventInstallFailed, map[string]string{"agentID": id, "error": message})
	i.finishTask(id, true)
}

func (i *Installer) finishTask(id string, success bool) {
	i.mu.Lock()
	defer i.mu.Unlock()
	delete(i.tasks, id)
	_ = success
}

// findBinary searches for name in PATH plus common installation directories.
func findBinary(name string) string {
	if path, err := exec.LookPath(name); err == nil {
		return path
	}
	for _, dir := range commonBinDirs() {
		if runtime.GOOS == "windows" {
			for _, candidate := range windowsCandidates(filepath.Join(dir, name)) {
				if isExecutable(candidate) {
					return candidate
				}
			}
		} else {
			candidate := filepath.Join(dir, name)
			if isExecutable(candidate) {
				return candidate
			}
		}
	}
	return ""
}

func windowsCandidates(base string) []string {
	pathext := os.Getenv("PATHEXT")
	if pathext == "" {
		pathext = ".exe;.cmd;.bat"
	}
	exts := strings.Split(pathext, string(filepath.ListSeparator))
	var out []string
	for _, ext := range exts {
		ext = strings.TrimSpace(ext)
		if ext == "" {
			continue
		}
		out = append(out, base+strings.ToLower(ext))
	}
	return out
}

func isExecutable(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	if info.IsDir() {
		return false
	}
	if runtime.GOOS == "windows" {
		return true
	}
	return info.Mode()&0o111 != 0
}

func commandVersion(name string) string {
	path := findBinary(name)
	if path == "" {
		return ""
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, path, "--version")
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	for _, line := range lines {
		if trimmed := strings.TrimSpace(line); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func commonBinDirs() []string {
	home, _ := os.UserHomeDir()
	var dirs []string

	if runtime.GOOS == "windows" {
		if home != "" {
			dirs = append(dirs,
				filepath.Join(home, "AppData", "Roaming", "npm"),
				filepath.Join(home, "AppData", "Local", "pnpm"),
				filepath.Join(home, "AppData", "Local", "Yarn", "bin"),
				filepath.Join(home, ".cargo", "bin"),
			)
		}
	} else {
		dirs = []string{
			"/opt/homebrew/bin",
			"/usr/local/bin",
			"/usr/bin",
			"/bin",
		}
		if home != "" {
			dirs = append(dirs,
				filepath.Join(home, ".local", "bin"),
				filepath.Join(home, ".npm-global", "bin"),
				filepath.Join(home, ".yarn", "bin"),
				filepath.Join(home, ".pnpm", "global", "node_modules", ".bin"),
				filepath.Join(home, ".cargo", "bin"),
			)
		}
	}

	for _, pm := range []string{"npm", "pnpm", "yarn"} {
		if bin := GlobalBinDir(pm); bin != "" {
			dirs = append(dirs, bin)
		}
	}
	return dirs
}

func augmentPath(pmPath string) string {
	path := os.Getenv("PATH")
	extra := commonBinDirs()
	if pmPath != "" {
		extra = append([]string{filepath.Dir(pmPath)}, extra...)
	}
	sep := string(filepath.ListSeparator)
	for _, dir := range extra {
		if !strings.Contains(sep+path+sep, sep+dir+sep) {
			path = dir + sep + path
		}
	}
	return path
}

func lastLines(s string, n int) string {
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	if len(lines) <= n {
		return strings.Join(lines, "\n")
	}
	return strings.Join(lines[len(lines)-n:], "\n")
}
