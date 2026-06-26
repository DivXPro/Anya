package adapters

import (
	"testing"

	"desktop/internal/acp"
)

func TestNewKimiAdapterInfo(t *testing.T) {
	a := NewKimiAdapter("sk-test", "moonshot-v1-32k")
	info := a.Info()
	if info.ID != "kimi" {
		t.Fatalf("unexpected id: %s", info.ID)
	}
	if info.Name != "Kimi" {
		t.Fatalf("unexpected name: %s", info.Name)
	}
	if info.Command != "kimi-api" {
		t.Fatalf("unexpected command: %s", info.Command)
	}
}

func TestKimiAdapterIsRunningWhenKeySet(t *testing.T) {
	a := NewKimiAdapter("sk-test", "")
	if !a.IsRunning() {
		t.Fatal("expected IsRunning=true when api key is set")
	}
}

func TestKimiAdapterIsRunningWhenKeyEmpty(t *testing.T) {
	a := NewKimiAdapter("", "")
	if a.IsRunning() {
		t.Fatal("expected IsRunning=false when api key is empty")
	}
}

func TestKimiAdapterLoadSessionUnit(t *testing.T) {
	a := NewKimiAdapter("", "")
	if err := a.LoadSession("session-kimi-test", nil); err != nil {
		t.Fatalf("load session: %v", err)
	}
	if a.CurrentSessionID() != "session-kimi-test" {
		t.Fatalf("expected session session-kimi-test, got %s", a.CurrentSessionID())
	}
}

func TestKimiAdapterDefaultModel(t *testing.T) {
	a := NewKimiAdapter("sk-test", "")
	info := a.Info()
	if info.ID != "kimi" {
		t.Fatalf("unexpected id: %s", info.ID)
	}
}

func TestKimiAdapterImplementsInterface(t *testing.T) {
	var _ acp.ACPAdapter = (*KimiAdapter)(nil)
}
