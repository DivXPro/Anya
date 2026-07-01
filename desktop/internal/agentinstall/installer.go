package agentinstall

import (
	"bytes"
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
	EventInstallProgress = "agent:install:progress"
)

// installTimeout bounds a single install/upgrade. Because the child runs in its
// own process group and is torn down via killProcessTree, this deadline is
// enforced even for `curl | bash`-style pipelines.
const installTimeout = 10 * time.Minute

// Installer detects and installs agent CLIs.
type Installer struct {
	app   *application.App
	db    *sql.DB
	mu    sync.Mutex
	tasks map[string]*Task
	// buildCmd builds the exec args + display string for an install/upgrade.
	// It defaults to buildAgentCommand and is overridden in tests to keep them
	// hermetic (the real builder would run the vendor installer over the network).
	buildCmd func(id string, isUpdate bool) ([]string, string, error)
}

type Task struct {
	AgentID string
	Running bool
	cancel  context.CancelFunc
}

func New(app *application.App, db *sql.DB) *Installer {
	return &Installer{
		app:      app,
		db:       db,
		tasks:    make(map[string]*Task),
		buildCmd: buildAgentCommand,
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

	if args, display, err := PlatformInstallCommand(id); err == nil {
		_ = args
		if err := store.UpdateAgentInstallCommand(i.db, id, display); err != nil {
			log.Printf("[agentinstall] save install command %s error: %v", id, err)
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

// AnyInstalling reports whether any agent install/upgrade is currently running.
// Used to block a self-update relaunch that would otherwise orphan the install.
func (i *Installer) AnyInstalling() bool {
	i.mu.Lock()
	defer i.mu.Unlock()
	for _, t := range i.tasks {
		if t.Running {
			return true
		}
	}
	return false
}

// Install starts an asynchronous install or in-place upgrade for the agent.
// If the CLI is already present the command is "anchored" to how it was
// installed (npm/pnpm/yarn vs vendor script); otherwise a fresh install is run.
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
	i.mu.Unlock()

	isUpdate := findBinary(info.Binary) != ""
	args, display, err := i.buildCmd(id, isUpdate)
	if err != nil {
		return err
	}

	if err := store.UpdateAgentInstallCommand(i.db, id, display); err != nil {
		log.Printf("[agentinstall] save install command %s error: %v", id, err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), installTimeout)

	i.mu.Lock()
	i.tasks[id] = &Task{AgentID: id, Running: true, cancel: cancel}
	i.mu.Unlock()

	i.emit(EventInstallStarted, map[string]string{"agentID": id})

	go func() {
		defer cancel()

		if len(args) == 0 {
			i.emitFailure(id, "empty install command")
			return
		}

		cmd := exec.CommandContext(ctx, args[0], args[1:]...)
		// Run in its own process group and tear down the whole tree on
		// cancel/timeout — otherwise `curl | bash` children (and any npm
		// postinstall) would escape the deadline and orphan.
		setProcAttr(cmd)
		cmd.Cancel = func() error { return killProcessTree(cmd) }
		// If the process ignores the kill (blocked in I/O), stop waiting after
		// a grace period so the goroutine can't hang forever.
		cmd.WaitDelay = 15 * time.Second

		// Ensure any discovered package manager and common bin dirs are on PATH.
		_, pmPath := DetectPackageManager()
		cmd.Env = append(os.Environ(), fmt.Sprintf("PATH=%s", augmentPath(pmPath)))

		// Stream combined output line-by-line: emit progress events for the UI
		// and keep a bounded tail for the failure message.
		collector := newProgressWriter(func(line string) {
			i.emit(EventInstallProgress, map[string]string{"agentID": id, "line": line})
		})
		cmd.Stdout = collector
		cmd.Stderr = collector

		log.Printf("[agentinstall] %s %s: %s", verb(isUpdate), id, display)
		runErr := cmd.Run()
		collector.flush()

		if runErr != nil {
			// Distinguish an explicit cancel from a genuine failure.
			if ctx.Err() == context.Canceled {
				i.emitFailure(id, "installation canceled")
				return
			}
			if ctx.Err() == context.DeadlineExceeded {
				i.emitFailure(id, fmt.Sprintf("installation timed out after %s", installTimeout))
				return
			}
			msg := strings.TrimSpace(collector.tail())
			if msg == "" {
				msg = runErr.Error()
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

func verb(isUpdate bool) string {
	if isUpdate {
		return "upgrading"
	}
	return "installing"
}

// Cancel aborts an in-progress install/upgrade for the agent by cancelling its
// context, which kills the whole process group. It is a no-op when nothing is
// running for that agent.
func (i *Installer) Cancel(id string) {
	i.mu.Lock()
	t, ok := i.tasks[id]
	var cancel context.CancelFunc
	if ok && t.Running {
		cancel = t.cancel
	}
	i.mu.Unlock()
	if cancel != nil {
		log.Printf("[agentinstall] cancel requested for %s", id)
		cancel()
	}
}

// Shutdown cancels every in-progress install so that no child process is left
// orphaned when the app quits or relaunches (e.g. during self-update).
func (i *Installer) Shutdown() {
	i.mu.Lock()
	cancels := make([]context.CancelFunc, 0, len(i.tasks))
	for _, t := range i.tasks {
		if t.Running && t.cancel != nil {
			cancels = append(cancels, t.cancel)
		}
	}
	i.mu.Unlock()
	for _, c := range cancels {
		c()
	}
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
	// Include the user's real login-shell PATH so binaries installed in
	// non-standard locations (e.g. ~/.kimi-code/bin, nvm/fnm, custom npm
	// prefixes) are found even when the app was launched from the Dock with a
	// minimal launchd PATH.
	dirs = append(dirs, loginShellPathDirs()...)
	return dedupeDirs(dirs)
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

var (
	shellPathOnce sync.Once
	shellPathDirs []string
)

// loginShellPathDirs returns the PATH directories from the user's login shell,
// resolved once and cached. macOS GUI apps launched from the Dock/Finder inherit
// a minimal launchd PATH that omits user-configured locations, so agent binaries
// installed there (e.g. ~/.kimi-code/bin) would otherwise go undetected.
func loginShellPathDirs() []string {
	shellPathOnce.Do(func() {
		shellPathDirs = resolveShellPathDirs()
	})
	return shellPathDirs
}

func resolveShellPathDirs() []string {
	if runtime.GOOS == "windows" {
		return nil
	}
	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "/bin/zsh" // macOS default login shell
	}
	const marker = "__ANYA_PATH__"
	// Sentinels let us extract $PATH even when rc files print banners.
	script := "printf '%s%s%s' '" + marker + "' \"$PATH\" '" + marker + "'"

	run := func(args ...string) (string, bool) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		out, err := exec.CommandContext(ctx, shell, args...).Output()
		if err != nil {
			return "", false
		}
		s := string(out)
		i := strings.Index(s, marker)
		j := strings.LastIndex(s, marker)
		if i < 0 || j <= i {
			return "", false
		}
		return s[i+len(marker) : j], true
	}

	// Login + interactive sources both profile and rc files (~/.zshrc etc., where
	// many tools add themselves); fall back to login-only if that form fails.
	pathValue, ok := run("-l", "-i", "-c", script)
	if !ok {
		pathValue, ok = run("-l", "-c", script)
		if !ok {
			return nil
		}
	}
	return filepath.SplitList(pathValue)
}

// dedupeDirs removes empty and duplicate entries, preserving first-seen order.
func dedupeDirs(dirs []string) []string {
	seen := make(map[string]struct{}, len(dirs))
	out := make([]string, 0, len(dirs))
	for _, d := range dirs {
		if d == "" {
			continue
		}
		if _, ok := seen[d]; ok {
			continue
		}
		seen[d] = struct{}{}
		out = append(out, d)
	}
	return out
}

// progressTailLines bounds how many recent output lines are retained for the
// failure message shown to the user.
const progressTailLines = 8

// progressWriter splits combined stdout/stderr into lines, emitting each
// non-empty line for live UI progress and retaining a bounded tail for error
// reporting. It is safe for the concurrent writes exec makes for stdout+stderr.
type progressWriter struct {
	mu    sync.Mutex
	buf   []byte
	lines []string
	emit  func(string)
}

func newProgressWriter(emit func(string)) *progressWriter {
	return &progressWriter{emit: emit}
}

func (w *progressWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.buf = append(w.buf, p...)
	for {
		idx := bytes.IndexByte(w.buf, '\n')
		if idx < 0 {
			break
		}
		line := string(w.buf[:idx])
		w.buf = append(w.buf[:0], w.buf[idx+1:]...)
		w.handleLine(line)
	}
	return len(p), nil
}

// handleLine records a trimmed, non-empty line and emits it. Caller holds mu.
func (w *progressWriter) handleLine(line string) {
	trimmed := strings.TrimSpace(strings.TrimRight(line, "\r"))
	if trimmed == "" {
		return
	}
	w.lines = append(w.lines, trimmed)
	if len(w.lines) > progressTailLines {
		w.lines = w.lines[len(w.lines)-progressTailLines:]
	}
	if w.emit != nil {
		w.emit(trimmed)
	}
}

// flush processes any trailing bytes not terminated by a newline.
func (w *progressWriter) flush() {
	w.mu.Lock()
	defer w.mu.Unlock()
	if len(w.buf) > 0 {
		line := string(w.buf)
		w.buf = w.buf[:0]
		w.handleLine(line)
	}
}

// tail returns the most recent retained output lines joined by newlines.
func (w *progressWriter) tail() string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return strings.Join(w.lines, "\n")
}
