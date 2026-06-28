package acp

import (
	"runtime"
	"testing"
	"time"
)

func longRunningCommand() string {
	if runtime.GOOS == "windows" {
		return "timeout /t 30"
	}
	return "sleep 30"
}

func TestProcessManagerStartStop(t *testing.T) {
	pm := NewProcessManager("go version")
	if err := pm.Start(); err != nil {
		t.Fatalf("start: %v", err)
	}
	if !pm.IsRunning() {
		t.Fatal("expected process to be running")
	}

	select {
	case <-pm.Done():
		// process exited immediately, that's ok for go version
	case <-time.After(500 * time.Millisecond):
	}

	if err := pm.Stop(); err != nil {
		t.Fatalf("stop: %v", err)
	}

	deadline := time.Now().Add(5 * time.Second)
	for pm.IsRunning() && time.Now().Before(deadline) {
		time.Sleep(50 * time.Millisecond)
	}
	if pm.IsRunning() {
		t.Fatal("process did not stop")
	}
}

func TestProcessManagerStopTerminatesLiveProcess(t *testing.T) {
	pm := NewProcessManager(longRunningCommand())
	if err := pm.Start(); err != nil {
		t.Fatalf("start: %v", err)
	}
	if !pm.IsRunning() {
		t.Fatal("expected process to be running")
	}

	time.Sleep(100 * time.Millisecond)

	if err := pm.Stop(); err != nil {
		t.Fatalf("stop: %v", err)
	}

	deadline := time.Now().Add(5 * time.Second)
	for pm.IsRunning() && time.Now().Before(deadline) {
		time.Sleep(50 * time.Millisecond)
	}
	if pm.IsRunning() {
		t.Fatal("live process did not stop")
	}
}
