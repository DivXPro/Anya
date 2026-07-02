package gateway

import (
	"bytes"
	"testing"

	"desktop/internal/acp"
)

func TestEncodeDecodeRoundTrip(t *testing.T) {
	msg := SummaryMessage("hello world")
	data, err := EncodeMessage(msg)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}

	evt, err := DecodeEvent(data[:len(data)-1])
	if err != nil {
		t.Fatalf("decode: %v", err)
	}

	if evt.Type != "summary" {
		t.Errorf("expected 'summary', got '%s'", evt.Type)
	}
}

func TestAllMessageTypes(t *testing.T) {
	msgs := []DeviceMessage{
		WelcomeMessage("dev1", "claude-code", "sess1", "desktop-test"),
		SummaryMessage("test summary"),
		TTSStartMessage("pcm"),
		TTSEndMessage(),
		StatusMessage("listening"),
		UIStateMessage("processing"),
		FirmwareVersionReqMessage(),
		FirmwareUpdateMessage("1.0.0", 1234, "abcd", 4096),
		FirmwareCommitMessage(),
		FirmwareUpdateCancelMessage(),
		AgentSessionListMessage("claude-code", []acp.AgentSession{{ID: "s1", Title: "One", CWD: "/tmp/one"}}),
		AgentSessionChangedMessage("claude-code", acp.AgentSession{ID: "s1", Title: "One", CWD: "/tmp/one"}),
		ConfirmMessage("req-1", "Allow access?", []ConfirmOption{{ID: "accept", Label: "Allow"}}),
		ConfirmCancelMessage("req-1"),
	}

	for i, msg := range msgs {
		data, err := EncodeMessage(msg)
		if err != nil {
			t.Errorf("msg[%d] encode: %v", i, err)
			continue
		}
		if len(data) == 0 {
			t.Errorf("msg[%d] encoded empty", i)
		}
	}
}

func TestAgentSessionListMessage(t *testing.T) {
	msg := AgentSessionListMessage("codex", []acp.AgentSession{{
		ID: "thread-1", Title: "Fix audio", CWD: "/Users/me/Elf", Source: "codex", CanResume: true,
	}})
	data, err := EncodeMessage(msg)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	if !bytes.Contains(data, []byte(`"type":"agent_session_list"`)) ||
		!bytes.Contains(data, []byte(`"agent_id":"codex"`)) ||
		!bytes.Contains(data, []byte(`"title":"Fix audio"`)) ||
		!bytes.Contains(data, []byte(`"cwd":"/Users/me/Elf"`)) {
		t.Fatalf("unexpected payload: %s", data)
	}
}

func TestDecodeButtonEvents(t *testing.T) {
	tests := []string{
		`{"type":"button","action":"push_to_talk"}`,
		`{"type":"button","action":"confirm"}`,
		`{"type":"ping"}`,
		`{"type":"audio_start","format":"pcm","sample_rate":16000}`,
	}

	for _, raw := range tests {
		evt, err := DecodeEvent([]byte(raw))
		if err != nil {
			t.Errorf("decode '%s': %v", raw, err)
			continue
		}
		if evt.Type == "" {
			t.Errorf("empty type for '%s'", raw)
		}
	}
}
