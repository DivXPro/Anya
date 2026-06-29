package adapters

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"desktop/internal/acp"
)

// acpHelperReply writes a single NDJSON JSON-RPC response to stdout.
func acpHelperReply(id int, result map[string]any) {
	b, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": id, "result": result})
	b = append(b, '\n')
	os.Stdout.Write(b)
}

// TestACPHelperProcess is not a real test: when re-executed with GO_ACP_HELPER=1
// it acts as a minimal NDJSON ACP agent over stdio. It answers initialize and
// session/new, and for session/load it records the received params to the file
// named by GO_ACP_CAPTURE so the parent test can assert on them. Under a normal
// `go test` run (without the env var) it returns immediately as a no-op.
func TestACPHelperProcess(t *testing.T) {
	if os.Getenv("GO_ACP_HELPER") != "1" {
		return
	}
	capturePath := os.Getenv("GO_ACP_CAPTURE")
	reader := bufio.NewReader(os.Stdin)
	for {
		line, err := reader.ReadBytes('\n')
		if err != nil {
			return
		}
		line = bytes.TrimSpace(line)
		if len(line) == 0 {
			continue
		}
		var req struct {
			ID     int             `json:"id"`
			Method string          `json:"method"`
			Params json.RawMessage `json:"params"`
		}
		if json.Unmarshal(line, &req) != nil {
			continue
		}
		switch req.Method {
		case "session/new":
			acpHelperReply(req.ID, map[string]any{"sessionId": "sess-new"})
		case "session/load":
			if capturePath != "" {
				_ = os.WriteFile(capturePath, []byte(req.Params), 0o644)
			}
			acpHelperReply(req.ID, map[string]any{"sessionId": "sess-loaded"})
		default:
			acpHelperReply(req.ID, map[string]any{"protocolVersion": 1})
		}
	}
}

type cwdLoadStopper interface {
	LoadSession(acpSessionID string, history []acp.Message) error
	SetCWD(cwd string)
	Stop() error
}

// TestLoadSessionSendsCwdAndMcpServers locks in the fix for the "invalid params"
// regression: ACP session/load requires cwd and mcpServers (same as session/new),
// not just sessionId. It drives each stdio-based adapter against a fake ACP agent
// and asserts the params that actually went over the wire.
func TestLoadSessionSendsCwdAndMcpServers(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("helper relies on an `sh -c` env prefix; unix only")
	}

	cases := []struct {
		name string
		make func(pm *acp.ProcessManager) cwdLoadStopper
	}{
		{"kimi", func(pm *acp.ProcessManager) cwdLoadStopper { a := NewKimiAdapter(); a.pm = pm; return a }},
		{"hermes", func(pm *acp.ProcessManager) cwdLoadStopper { a := NewHermesAdapter(); a.pm = pm; return a }},
		{"opencode", func(pm *acp.ProcessManager) cwdLoadStopper { a := NewOpenCodeAdapter(); a.pm = pm; return a }},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			capture := filepath.Join(t.TempDir(), "load-params.json")
			cmd := fmt.Sprintf("GO_ACP_HELPER=1 GO_ACP_CAPTURE=%q %q -test.run=^TestACPHelperProcess$",
				capture, os.Args[0])
			pm := acp.NewProcessManagerWithFraming(cmd, acp.NDJSONFraming)

			a := c.make(pm)
			defer a.Stop()
			a.SetCWD("/tmp/elf-test-ws")

			if err := a.LoadSession("sess-123", nil); err != nil {
				t.Fatalf("LoadSession: %v", err)
			}

			raw, err := os.ReadFile(capture)
			if err != nil {
				t.Fatalf("session/load was not received by the agent (no capture file): %v", err)
			}
			var params map[string]any
			if err := json.Unmarshal(raw, &params); err != nil {
				t.Fatalf("unmarshal captured params: %v", err)
			}

			if _, ok := params["cwd"]; !ok {
				t.Errorf("session/load params missing cwd: %s", raw)
			}
			if _, ok := params["mcpServers"]; !ok {
				t.Errorf("session/load params missing mcpServers: %s", raw)
			}
			if params["sessionId"] != "sess-123" {
				t.Errorf("session/load sessionId = %v, want sess-123", params["sessionId"])
			}
			if params["cwd"] != "/tmp/elf-test-ws" {
				t.Errorf("session/load cwd = %v, want /tmp/elf-test-ws", params["cwd"])
			}
		})
	}
}
