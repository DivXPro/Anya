package acp

type ACPAdapter interface {
	Send(prompt string, history []Message) (<-chan StreamEvent, error)
	Info() AgentInfo
	IsRunning() bool
	Stop() error
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
