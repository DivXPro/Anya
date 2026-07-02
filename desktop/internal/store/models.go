package store

type Dialogue struct {
	ID                   string  `json:"id"`
	DeviceID             string  `json:"device_id"`
	AgentID              string  `json:"agent_id"`
	AgentSessionID       *string `json:"agent_session_id"`
	AgentSessionProvider *string `json:"agent_session_provider"`
	ACPSessionID         *string `json:"acp_session_id,omitempty"`
	ACPAgentID           *string `json:"acp_agent_id,omitempty"`
	CreatedAt            string  `json:"created_at"`
	ClosedAt             *string `json:"closed_at"`
}

// Session is kept as a transition alias while application call sites move to
// Dialogue terminology.
type Session = Dialogue

type Message struct {
	ID         string  `json:"id"`
	DialogueID string  `json:"dialogue_id"`
	SessionID  string  `json:"session_id,omitempty"`
	Role       string  `json:"role"`
	Content    string  `json:"content"`
	AudioURL   *string `json:"audio_url"`
	Summary    *string `json:"summary"`
	CreatedAt  string  `json:"created_at"`
}

type Agent struct {
	ID             string  `json:"id"`
	Name           string  `json:"name"`
	Command        string  `json:"command"`
	Enabled        bool    `json:"enabled"`
	Selected       bool    `json:"selected"`
	Version        *string `json:"version"`
	Config         *string `json:"config"`
	Installed      bool    `json:"installed"`
	InstallCommand *string `json:"install_command"`
}
