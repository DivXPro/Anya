package appupdate

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
)

// ErrUpdateInProgress is returned by DownloadAndApply when a download/apply is
// already running, so callers can distinguish a duplicate trigger from a real
// failure (e.g. to avoid re-enabling UI that the active download still owns).
var ErrUpdateInProgress = errors.New("update already in progress")

// Manager orchestrates check → download → verify → apply → relaunch.
type Manager struct {
	checker  *Checker
	verifier *Verifier
	applier  Applier
	emit     Emitter
	http     *http.Client
	current  string

	mu        sync.Mutex
	state     State
	available *UpdateInfo

	inProgress atomic.Bool
}

func NewManager(current string, checker *Checker, verifier *Verifier, applier Applier, emit Emitter) *Manager {
	return &Manager{
		checker:  checker,
		verifier: verifier,
		applier:  applier,
		emit:     emit,
		http:     &http.Client{},
		current:  current,
		state:    StateIdle,
	}
}

func (m *Manager) setState(s State) { m.mu.Lock(); m.state = s; m.mu.Unlock() }

// State returns the current state-machine value.
func (m *Manager) State() State { m.mu.Lock(); defer m.mu.Unlock(); return m.state }

// Available returns the update found by the most recent successful check, or nil
// when none is cached. It performs no network I/O so the UI can query it cheaply
// (e.g. to render an "update available" indicator after a background check).
func (m *Manager) Available() *UpdateInfo {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.available
}

// InProgress reports whether a download/apply is currently running.
func (m *Manager) InProgress() bool { return m.inProgress.Load() }

// CheckForUpdate queries for a newer release. Returns (nil, nil) when up to date.
func (m *Manager) CheckForUpdate(ctx context.Context) (*UpdateInfo, error) {
	if m.inProgress.Load() {
		return nil, nil
	}
	m.setState(StateChecking)
	info, err := m.checker.Latest(ctx)
	if err != nil {
		m.setState(StateError)
		return nil, err
	}
	newer, err := IsNewer(info.Version, m.current)
	if err != nil {
		m.setState(StateError)
		return nil, err
	}
	if !newer {
		m.setState(StateUpToDate)
		return nil, nil
	}
	m.mu.Lock()
	m.available = info
	m.state = StateAvailable
	m.mu.Unlock()
	m.emitSafe(EventAvailable, info)
	return info, nil
}

// DownloadAndApply downloads the available update, verifies it, applies it, relaunches.
func (m *Manager) DownloadAndApply(ctx context.Context) error {
	if !m.inProgress.CompareAndSwap(false, true) {
		return ErrUpdateInProgress
	}
	defer m.inProgress.Store(false)

	m.mu.Lock()
	info := m.available
	m.mu.Unlock()
	if info == nil {
		return fmt.Errorf("no update available; call CheckForUpdate first")
	}

	tmp, err := os.MkdirTemp("", "anya-update-*")
	if err != nil {
		return m.failf("create temp: %w", err)
	}
	defer os.RemoveAll(tmp)

	assetPath := filepath.Join(tmp, info.AssetName)
	m.setState(StateDownloading)
	if err := m.download(ctx, info.AssetURL, assetPath, info.Size); err != nil {
		return m.failf("download asset: %w", err)
	}
	checksums, err := m.downloadBytes(ctx, info.ChecksumsURL)
	if err != nil {
		return m.failf("download checksums: %w", err)
	}
	sig, err := m.downloadBytes(ctx, info.SignatureURL)
	if err != nil {
		return m.failf("download signature: %w", err)
	}

	m.setState(StateVerifying)
	if err := m.verifier.VerifySignature(checksums, sig); err != nil {
		return m.failf("verify signature: %w", err)
	}
	sum, err := ChecksumFor(checksums, info.AssetName)
	if err != nil {
		return m.failf("%w", err)
	}
	if err := VerifyFileSHA256(assetPath, sum); err != nil {
		return m.failf("verify asset: %w", err)
	}

	m.setState(StateApplying)
	m.emitSafe(EventApplying, nil)
	if err := m.applier.Apply(assetPath); err != nil {
		return m.failf("apply: %w", err)
	}
	if err := m.applier.Relaunch(); err != nil {
		return m.failf("relaunch: %w", err)
	}
	return nil
}

func (m *Manager) emitSafe(name string, data any) {
	if m.emit != nil {
		m.emit.Emit(name, data)
	}
}

func (m *Manager) failf(format string, args ...any) error {
	err := fmt.Errorf(format, args...)
	m.setState(StateError)
	m.emitSafe(EventError, map[string]string{"message": err.Error()})
	return err
}

func (m *Manager) download(ctx context.Context, url, dst string, total int64) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	resp, err := m.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("status %d", resp.StatusCode)
	}
	f, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer f.Close()
	pw := &progressWriter{total: total, emit: m.emit}
	_, err = io.Copy(io.MultiWriter(f, pw), resp.Body)
	return err
}

func (m *Manager) downloadBytes(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := m.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status %d", resp.StatusCode)
	}
	const maxMetaBytes = 1 << 20 // 1 MiB ceiling for checksums/signature
	return io.ReadAll(io.LimitReader(resp.Body, maxMetaBytes))
}

type progressWriter struct {
	total   int64
	written int64
	last    int
	emit    Emitter
}

func (p *progressWriter) Write(b []byte) (int, error) {
	n := len(b)
	p.written += int64(n)
	if p.total > 0 && p.emit != nil {
		pct := int(p.written * 100 / p.total)
		if pct != p.last {
			p.last = pct
			p.emit.Emit(EventProgress, map[string]int{"percent": pct})
		}
	}
	return n, nil
}
