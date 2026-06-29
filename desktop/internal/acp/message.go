package acp

import "fmt"

type StreamEvent struct {
	Type       string `json:"type"`
	Content    string `json:"content"`
	ToolCallID string `json:"toolCallId,omitempty"`
	ToolName   string `json:"toolName,omitempty"`
	Error      error  `json:"-"`
}

func (e StreamEvent) IsContent() bool   { return e.Type == "text_delta" || e.Type == "ask_user" }
func (e StreamEvent) IsDone() bool      { return e.Type == "done" }
func (e StreamEvent) IsError() bool     { return e.Type == "error" }
func (e StreamEvent) IsSkippable() bool { return e.Type == "thinking" || e.Type == "tool_use" }
func (e StreamEvent) IsAskUser() bool   { return e.Type == "ask_user" }

func (e StreamEvent) String() string {
	if e.Error != nil {
		return fmt.Sprintf("StreamEvent{%s, error=%v}", e.Type, e.Error)
	}
	return fmt.Sprintf("StreamEvent{%s, len=%d}", e.Type, len(e.Content))
}

type PermissionOption struct {
	ID    string
	Label string
}

type PermissionRequest struct {
	ID      string
	Prompt  string
	Options []PermissionOption
}

type PermissionResponder interface {
	PermissionRequests() <-chan PermissionRequest
	RespondPermission(requestID, optionID string) error
}
