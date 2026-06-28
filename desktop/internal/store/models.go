package store

type Session struct {
	ID           string  `json:"id"`
	DeviceID     string  `json:"device_id"`
	AgentID      string  `json:"agent_id"`
	ACPSessionID *string `json:"acp_session_id"`
	ACPAgentID   *string `json:"acp_agent_id"`
	CreatedAt    string  `json:"created_at"`
	ClosedAt     *string `json:"closed_at"`
}

type Message struct {
	ID        string  `json:"id"`
	SessionID string  `json:"session_id"`
	Role      string  `json:"role"`
	Content   string  `json:"content"`
	AudioURL  *string `json:"audio_url"`
	Summary   *string `json:"summary"`
	CreatedAt string  `json:"created_at"`
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
