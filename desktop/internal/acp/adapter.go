package acp

type ACPAdapter interface {
	Send(prompt string, history []Message) (<-chan StreamEvent, error)
	LoadSession(acpSessionID string, history []Message) error
	CurrentSessionID() string
	Info() AgentInfo
	IsRunning() bool
	Stop() error
	SetCWD(cwd string) // NEW
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
