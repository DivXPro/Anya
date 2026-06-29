package acp

import "testing"

type fakeAdapter struct {
	ACPAdapter
	id  string
	cwd string
}

func (f *fakeAdapter) Info() AgentInfo {
	return AgentInfo{ID: f.id, Name: "Fake", Command: "fake"}
}

func (f *fakeAdapter) SetCWD(cwd string) {
	f.cwd = cwd
}

func TestRouterSetCWD(t *testing.T) {
	r := NewRouter()
	a1 := &fakeAdapter{ACPAdapter: &mockAdapter{}, id: "agent-a"}
	a2 := &fakeAdapter{ACPAdapter: &mockAdapter{}, id: "agent-b"}

	r.Register(a1)
	r.Register(a2)

	r.SetCWD("/tmp/workspace")

	if a1.cwd != "/tmp/workspace" {
		t.Fatalf("expected adapter a1 cwd '/tmp/workspace', got %q", a1.cwd)
	}
	if a2.cwd != "/tmp/workspace" {
		t.Fatalf("expected adapter a2 cwd '/tmp/workspace', got %q", a2.cwd)
	}
}
