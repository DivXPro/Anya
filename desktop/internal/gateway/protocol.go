package gateway

import (
	"encoding/json"

	"desktop/internal/acp"
)

type DeviceMessage struct {
	Type    string                 `json:"type"`
	Text    string                 `json:"text,omitempty"`
	Format  string                 `json:"format,omitempty"`
	State   string                 `json:"state,omitempty"`
	Payload map[string]interface{} `json:"payload,omitempty"`
}

type DeviceEvent struct {
	Type    string                 `json:"type"`
	Format  string                 `json:"format,omitempty"`
	Action  string                 `json:"action,omitempty"`
	Payload map[string]interface{} `json:"payload,omitempty"`
}

func EncodeMessage(msg DeviceMessage) ([]byte, error) {
	data, err := json.Marshal(msg)
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}

func DecodeEvent(data []byte) (DeviceEvent, error) {
	var evt DeviceEvent
	if err := json.Unmarshal(data, &evt); err != nil {
		return evt, err
	}
	return evt, nil
}

func WelcomeMessage(deviceID, agentID, sessionID, desktopID string) DeviceMessage {
	return DeviceMessage{
		Type: "session",
		Payload: map[string]interface{}{
			"device_id":  deviceID,
			"agent_id":   agentID,
			"session_id": sessionID,
			"desktop_id": desktopID,
		},
	}
}

func SummaryMessage(text string) DeviceMessage {
	return DeviceMessage{Type: "summary", Text: text}
}

func TTSStartMessage(format string) DeviceMessage {
	return DeviceMessage{Type: "tts_start", Format: format}
}

func TTSEndMessage() DeviceMessage {
	return DeviceMessage{Type: "tts_end"}
}

func StatusMessage(state string) DeviceMessage {
	return DeviceMessage{Type: "status", State: state}
}

func UIStateMessage(state string) DeviceMessage {
	return DeviceMessage{Type: "ui_state", State: state}
}

func AgentSessionListMessage(agentID string, sessions []acp.AgentSession) DeviceMessage {
	items := make([]map[string]interface{}, len(sessions))
	for i, s := range sessions {
		items[i] = map[string]interface{}{
			"id":         s.ID,
			"title":      s.Title,
			"cwd":        s.CWD,
			"updated_at": s.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
			"source":     s.Source,
			"can_resume": s.CanResume,
		}
	}
	return DeviceMessage{
		Type: "agent_session_list",
		Payload: map[string]interface{}{
			"agent_id":   agentID,
			"sessions":   items,
			"can_create": true,
		},
	}
}

func AgentSessionChangedMessage(agentID string, session acp.AgentSession) DeviceMessage {
	return DeviceMessage{
		Type: "agent_session_changed",
		Payload: map[string]interface{}{
			"agent_id":    agentID,
			"id":          session.ID,
			"title":       session.Title,
			"cwd":         session.CWD,
			"source":      session.Source,
			"can_resume":  session.CanResume,
			"updated_at":  session.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
			"new_session": session.ID == "",
		},
	}
}

func FirmwareVersionReqMessage() DeviceMessage {
	return DeviceMessage{Type: "firmware_version_req"}
}

func FirmwareUpdateMessage(version string, size int, md5 string, chunkSize int) DeviceMessage {
	return DeviceMessage{
		Type: "firmware_update",
		Payload: map[string]interface{}{
			"version":    version,
			"size":       size,
			"md5":        md5,
			"chunk_size": chunkSize,
		},
	}
}

func FirmwareCommitMessage() DeviceMessage {
	return DeviceMessage{Type: "firmware_commit"}
}

func FirmwareUpdateCancelMessage() DeviceMessage {
	return DeviceMessage{Type: "firmware_update_cancel"}
}

type ConfirmOption struct {
	ID    string `json:"id"`
	Label string `json:"label"`
}

func ConfirmMessage(requestID, text string, options []ConfirmOption) DeviceMessage {
	opts := make([]map[string]interface{}, len(options))
	for i, o := range options {
		opts[i] = map[string]interface{}{"id": o.ID, "label": o.Label}
	}
	return DeviceMessage{
		Type: "confirm",
		Payload: map[string]interface{}{
			"request_id": requestID,
			"text":       text,
			"options":    opts,
		},
	}
}

func ConfirmCancelMessage(requestID string) DeviceMessage {
	return DeviceMessage{
		Type: "confirm_cancel",
		Payload: map[string]interface{}{
			"request_id": requestID,
		},
	}
}
