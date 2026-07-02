package acp

import "time"

type ACPAdapter interface {
	Send(prompt string, history []Message) (<-chan StreamEvent, error)
	LoadSession(acpSessionID string, history []Message) error
	CurrentSessionID() string
	Info() AgentInfo
	IsRunning() bool
	Stop() error
	SetCWD(cwd string) // NEW
}

type AgentSession struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	CWD       string    `json:"cwd"`
	UpdatedAt time.Time `json:"updated_at"`
	Source    string    `json:"source"`
	CanResume bool      `json:"can_resume"`
}

type AgentSessionProvider interface {
	ListAgentSessions(limit int) ([]AgentSession, error)
}

type AgentSessionLoader interface {
	LoadAgentSession(id, cwd string) error
}

type AgentSessionStarter interface {
	StartNewAgentSession(cwd string) error
}

type AgentInfo struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Command string `json:"command"`
}

type Capability struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}
