package acp

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestProcessManagerFramingDefaults(t *testing.T) {
	lsp := NewProcessManager("echo test")
	if lsp.framing != LSPFraming {
		t.Fatalf("expected default LSP framing, got %v", lsp.framing)
	}

	ndjson := NewProcessManagerWithFraming("echo test", NDJSONFraming)
	if ndjson.framing != NDJSONFraming {
		t.Fatalf("expected NDJSON framing, got %v", ndjson.framing)
	}
}

func TestNDJSONFramingRoundTrip(t *testing.T) {
	messages := []map[string]any{
		{"jsonrpc": "2.0", "id": 1, "method": "initialize", "params": map[string]any{"protocolVersion": 1}},
		{"jsonrpc": "2.0", "method": "session/update", "params": map[string]any{"update": map[string]any{"type": "agent_message_chunk", "text": "hello"}}},
	}

	var buf bytes.Buffer
	for _, msg := range messages {
		data, err := json.Marshal(msg)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		buf.Write(data)
		buf.WriteByte('\n')
	}

	var parsed []map[string]any
	rest := buf.Bytes()
	for len(rest) > 0 {
		idx := bytes.IndexByte(rest, '\n')
		if idx < 0 {
			break
		}
		line := bytes.TrimSpace(rest[:idx])
		rest = rest[idx+1:]
		if len(line) == 0 {
			continue
		}
		var msg map[string]any
		if err := json.Unmarshal(line, &msg); err != nil {
			t.Fatalf("parse line %q: %v", line, err)
		}
		parsed = append(parsed, msg)
	}

	if len(parsed) != len(messages) {
		t.Fatalf("expected %d messages, got %d", len(messages), len(parsed))
	}
	if parsed[0]["method"] != "initialize" {
		t.Fatalf("expected initialize, got %v", parsed[0]["method"])
	}
}
