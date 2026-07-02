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

type fakeAgentSessionAdapter struct {
	*fakeAdapter
	sessions   []AgentSession
	loadedID   string
	startedCWD string
}

func (f *fakeAgentSessionAdapter) ListAgentSessions(limit int) ([]AgentSession, error) {
	if len(f.sessions) > limit {
		return f.sessions[:limit], nil
	}
	return f.sessions, nil
}

func (f *fakeAgentSessionAdapter) LoadAgentSession(id, cwd string) error {
	f.loadedID = id
	f.SetCWD(cwd)
	return nil
}

func (f *fakeAgentSessionAdapter) StartNewAgentSession(cwd string) error {
	f.startedCWD = cwd
	f.SetCWD(cwd)
	return nil
}

func TestRouterAgentSessionProvider(t *testing.T) {
	r := NewRouter()
	adapter := &fakeAgentSessionAdapter{
		fakeAdapter: &fakeAdapter{ACPAdapter: &mockAdapter{}, id: "mock"},
		sessions: []AgentSession{
			{ID: "s1", Title: "One", CWD: "/tmp/one"},
			{ID: "s2", Title: "Two", CWD: "/tmp/two"},
		},
	}
	r.Register(adapter)

	got, err := r.ListAgentSessions("mock", 1)
	if err != nil {
		t.Fatalf("list agent sessions: %v", err)
	}
	if len(got) != 1 || got[0].ID != "s1" {
		t.Fatalf("unexpected sessions: %+v", got)
	}

	if err := r.LoadAgentSession("mock", "s1", "/tmp/one"); err != nil {
		t.Fatalf("load agent session: %v", err)
	}
	if adapter.loadedID != "s1" || adapter.cwd != "/tmp/one" {
		t.Fatalf("load not forwarded: loaded=%q cwd=%q", adapter.loadedID, adapter.cwd)
	}

	if err := r.StartNewAgentSession("mock", "/tmp/new"); err != nil {
		t.Fatalf("start new agent session: %v", err)
	}
	if adapter.startedCWD != "/tmp/new" || adapter.cwd != "/tmp/new" {
		t.Fatalf("start not forwarded: started=%q cwd=%q", adapter.startedCWD, adapter.cwd)
	}
}
