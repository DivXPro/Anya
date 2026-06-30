package adapters

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"desktop/internal/acp"
)

// stressAdapter is the subset of behavior the concurrency/contract tests drive.
// kimi, hermes and opencode share the same stdio + session/new + session/prompt
// shape, so they run through the same fake-agent harness.
type stressAdapter interface {
	Send(prompt string, history []acp.Message) (<-chan acp.StreamEvent, error)
	LoadSession(acpSessionID string, history []acp.Message) error
	CurrentSessionID() string
	SetCWD(cwd string)
	Stop() error
}

var stressAdapters = []struct {
	name string
	make func(pm *acp.ProcessManager) stressAdapter
}{
	{"kimi", func(pm *acp.ProcessManager) stressAdapter { a := NewKimiAdapter(); a.pm = pm; return a }},
	{"hermes", func(pm *acp.ProcessManager) stressAdapter { a := NewHermesAdapter(); a.pm = pm; return a }},
	{"opencode", func(pm *acp.ProcessManager) stressAdapter { a := NewOpenCodeAdapter(); a.pm = pm; return a }},
}

// helperCmd builds the shell command that re-execs the test binary as the fake
// ACP agent (see TestACPHelperProcess). extraEnv is prepended verbatim (e.g.
// `GO_ACP_LOG=... `).
func helperCmd(extraEnv string) string {
	return fmt.Sprintf("%sGO_ACP_HELPER=1 %q -test.run=^TestACPHelperProcess$", extraEnv, os.Args[0])
}

// drainEvents consumes a Send stream until it closes or the deadline passes.
// Concurrent Send calls share one activeStream, so a stream may legitimately be
// superseded and never closed; the deadline keeps the drainer from blocking.
func drainEvents(ch <-chan acp.StreamEvent, d time.Duration) {
	deadline := time.After(d)
	for {
		select {
		case _, ok := <-ch:
			if !ok {
				return
			}
		case <-deadline:
			return
		}
	}
}

// TestAdapterConcurrentNoDeadlockOrRace hammers an adapter with concurrent
// Send / LoadSession / CurrentSessionID while its dispatch loop runs, against a
// fake ACP agent. Run under -race it guards two regressions at once:
//   - a data race on the a.mu-guarded fields (sessionID etc.), which -race flags
//     directly; and
//   - a deadlock from holding a.mu across a blocking sendRequest/SendJSON — the
//     class of bug that broke the Claude adapter. Such a bug would stall every
//     a.mu user (the dispatch loop can't deliver the response the blocked call
//     is waiting for), and the 30s watchdog below fires.
func TestAdapterConcurrentNoDeadlockOrRace(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("helper relies on an `sh -c` env prefix; unix only")
	}
	for _, c := range stressAdapters {
		c := c
		t.Run(c.name, func(t *testing.T) {
			pm := acp.NewProcessManagerWithFraming(helperCmd(""), acp.NDJSONFraming)
			a := c.make(pm)
			defer a.Stop()
			a.SetCWD("/ws")

			// Warm up: start the process, the dispatch loop, and a session before
			// the concurrent storm.
			ch, err := a.Send("warmup", nil)
			if err != nil {
				t.Fatalf("warmup Send: %v", err)
			}
			drainEvents(ch, 5*time.Second)

			const iters = 30
			var wg sync.WaitGroup
			errCh := make(chan error, 16)
			fail := func(err error) {
				select {
				case errCh <- err:
				default:
				}
			}

			// Serialized sender (the app serializes same-session turns via its
			// turn guard; here we model that while keeping the dispatch loop hot).
			wg.Add(1)
			go func() {
				defer wg.Done()
				for i := 0; i < iters; i++ {
					ch, err := a.Send("hi", nil)
					if err != nil {
						fail(fmt.Errorf("Send: %w", err))
						return
					}
					drainEvents(ch, 3*time.Second)
				}
			}()

			// Concurrent readers of the sessionID field.
			for g := 0; g < 3; g++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					for i := 0; i < iters*5; i++ {
						_ = a.CurrentSessionID()
					}
				}()
			}

			// Concurrent LoadSession: each call uses sendRequest, so a
			// lock-held-across-sendRequest bug would deadlock here (caught by the
			// watchdog). Distinct ids force a real request every time.
			wg.Add(1)
			go func() {
				defer wg.Done()
				for i := 0; i < iters; i++ {
					if err := a.LoadSession(fmt.Sprintf("sess-%d", i), nil); err != nil {
						fail(fmt.Errorf("LoadSession: %w", err))
						return
					}
				}
			}()

			done := make(chan struct{})
			go func() { wg.Wait(); close(done) }()

			select {
			case <-done:
			case <-time.After(30 * time.Second):
				t.Fatal("operations did not finish in 30s — possible deadlock (a lock held across a blocking call?)")
			}
			close(errCh)
			for err := range errCh {
				t.Error(err)
			}
		})
	}
}

type rpcEntry struct {
	Method string          `json:"method"`
	Raw    json.RawMessage `json:"params"`
	params map[string]interface{}
}

func readRPCLog(t *testing.T, path string) []rpcEntry {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read rpc log %s: %v", path, err)
	}
	var entries []rpcEntry
	for _, line := range strings.Split(strings.TrimSpace(string(raw)), "\n") {
		if line == "" {
			continue
		}
		var e rpcEntry
		if err := json.Unmarshal([]byte(line), &e); err != nil {
			t.Fatalf("parse rpc log line %q: %v", line, err)
		}
		_ = json.Unmarshal(e.Raw, &e.params)
		entries = append(entries, e)
	}
	return entries
}

// TestAdapterReusesSessionAndDoesNotResendCwdEachTurn locks in the session/cwd
// contract: the session is created once and reused across turns, so cwd (and
// mcpServers) are sent only on session/new and are NOT re-submitted on every
// session/prompt. A regression that recreated the session per turn — forcing
// cwd to be re-sent each time and losing conversation continuity — fails here.
func TestAdapterReusesSessionAndDoesNotResendCwdEachTurn(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("helper relies on an `sh -c` env prefix; unix only")
	}
	for _, c := range stressAdapters {
		c := c
		t.Run(c.name, func(t *testing.T) {
			logf := filepath.Join(t.TempDir(), "rpc.log")
			pm := acp.NewProcessManagerWithFraming(
				helperCmd(fmt.Sprintf("GO_ACP_LOG=%q ", logf)), acp.NDJSONFraming)
			a := c.make(pm)
			defer a.Stop()
			a.SetCWD("/ws-A")

			const turns = 3
			for i := 0; i < turns; i++ {
				ch, err := a.Send("hi", nil)
				if err != nil {
					t.Fatalf("Send #%d: %v", i, err)
				}
				drainEvents(ch, 5*time.Second)
			}

			entries := readRPCLog(t, logf)
			var newCount, promptCount int
			var newParams map[string]interface{}
			for _, e := range entries {
				switch e.Method {
				case "session/new":
					newCount++
					newParams = e.params
				case "session/prompt":
					promptCount++
					if _, ok := e.params["cwd"]; ok {
						t.Errorf("session/prompt unexpectedly carried cwd (should reuse the session): %v", e.params)
					}
				}
			}

			if newCount != 1 {
				t.Errorf("session/new sent %d times, want 1 (session must be reused, cwd not resent each turn)", newCount)
			}
			if promptCount != turns {
				t.Errorf("session/prompt sent %d times, want %d", promptCount, turns)
			}
			if newParams == nil {
				t.Fatalf("no session/new observed in rpc log")
			}
			if newParams["cwd"] != "/ws-A" {
				t.Errorf("session/new cwd = %v, want /ws-A", newParams["cwd"])
			}
			if _, ok := newParams["mcpServers"]; !ok {
				t.Errorf("session/new missing mcpServers: %v", newParams)
			}
		})
	}
}
