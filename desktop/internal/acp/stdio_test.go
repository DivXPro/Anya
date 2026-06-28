package acp

import "testing"

func TestProcessManagerFramingDefaults(t *testing.T) {
	lsp := NewProcessManager("go version")
	if lsp.framing != LSPFraming {
		t.Errorf("expected LSPFraming, got %d", lsp.framing)
	}

	ndjson := NewProcessManagerWithFraming("go version", NDJSONFraming)
	if ndjson.framing != NDJSONFraming {
		t.Errorf("expected NDJSONFraming, got %d", ndjson.framing)
	}
}

func TestProcessManagerEmptyCommand(t *testing.T) {
	pm := NewProcessManager("   ")
	if err := pm.Start(); err == nil {
		t.Fatal("expected error for empty command")
	}
}
