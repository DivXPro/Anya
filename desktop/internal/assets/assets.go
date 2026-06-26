package assets

import (
	"embed"
	"fmt"
)

//go:embed *.icns
var logosFS embed.FS

var logoNames = map[string]string{
	"claude-code": "claude.icns",
	"opencode":    "opencode.icns",
}

// AgentLogo returns the embedded logo bytes for the given agent id.
// The bytes are in macOS .icns format so NSImage can pick the right
// resolution (16x16 points, 32x32@2x on Retina).
// It returns nil, nil if no logo is available.
func AgentLogo(agentID string) ([]byte, error) {
	name, ok := logoNames[agentID]
	if !ok {
		return nil, nil
	}
	b, err := logosFS.ReadFile(name)
	if err != nil {
		return nil, fmt.Errorf("read logo %s: %w", name, err)
	}
	return b, nil
}

// AgentLogoPath returns the embed file name for the agent, or empty string.
func AgentLogoPath(agentID string) string {
	return logoNames[agentID]
}
