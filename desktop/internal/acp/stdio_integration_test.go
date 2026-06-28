//go:build integration

package acp

import (
	"encoding/json"
	"os/exec"
	"testing"
	"time"
)

func TestProcessManagerNDJSONWithOpenCode(t *testing.T) {
	if _, err := exec.LookPath("opencode"); err != nil {
		t.Skip("opencode not installed")
	}

	pm := NewProcessManagerWithFraming("opencode acp", NDJSONFraming)
	if err := pm.Start(); err != nil {
		t.Fatalf("start opencode: %v", err)
	}
	defer pm.Stop()

	init := map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "initialize",
		"params": map[string]any{
			"protocolVersion": 1,
			"clientCapabilities": map[string]any{
				"fs":       map[string]bool{"readTextFile": true, "writeTextFile": true},
				"terminal": true,
			},
			"clientInfo": map[string]string{"name": "anya-test", "version": "1.0.0"},
		},
	}
	if err := pm.SendJSON(init); err != nil {
		t.Fatalf("send initialize: %v", err)
	}

	select {
	case raw := <-pm.Events():
		var resp struct {
			ID     int `json:"id"`
			Result struct {
				ProtocolVersion int `json:"protocolVersion"`
			} `json:"result"`
		}
		if err := json.Unmarshal(raw, &resp); err != nil {
			t.Fatalf("unmarshal response: %v", err)
		}
		if resp.ID != 1 || resp.Result.ProtocolVersion != 1 {
			t.Fatalf("unexpected response: %s", string(raw))
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for initialize response")
	}
}
