package adapters

import (
	"encoding/json"
	"fmt"
	"log"
	"os/exec"
	"strconv"
	"sync"
	"time"

	"desktop/internal/acp"
)

type PiAdapter struct {
	pm              *acp.ProcessManager
	info            acp.AgentInfo
	mu              sync.Mutex
	dispatchMu      sync.Mutex
	dispatchRunning bool
	reqID           int
	pending         map[string]chan acp.StreamEvent
	activeStream    chan acp.StreamEvent
	streamMu        sync.RWMutex
	sessionID       string
	systemPrompt    string
}

func NewPiAdapter() *PiAdapter {
	return &PiAdapter{
		info: acp.AgentInfo{
			ID:      "pi",
			Name:    "Pi",
			Command: "pi --mode rpc --no-session --exclude-tools ask_question",
		},
		pm:           acp.NewProcessManagerWithFraming("pi --mode rpc --no-session --exclude-tools ask_question", acp.NDJSONFraming),
		pending:      make(map[string]chan acp.StreamEvent),
		systemPrompt: DefaultSystemPrompt,
	}
}

func (a *PiAdapter) SetSystemPrompt(prompt string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.systemPrompt = prompt
}

func (a *PiAdapter) ensureInit() error {
	if !a.pm.IsRunning() {
		if err := a.pm.Start(); err != nil {
			return fmt.Errorf("start pi: %w", err)
		}
	}

	a.dispatchMu.Lock()
	if !a.dispatchRunning {
		a.dispatchRunning = true
		go a.dispatchLoop(a.pm)
	}
	a.dispatchMu.Unlock()
	return nil
}

func (a *PiAdapter) Send(prompt string, history []acp.Message) (<-chan acp.StreamEvent, error) {
	if err := a.ensureInit(); err != nil {
		return nil, err
	}

	a.mu.Lock()
	a.reqID++
	id := a.reqID
	systemPrompt := a.systemPrompt
	ch := make(chan acp.StreamEvent, 256)
	a.streamMu.Lock()
	a.activeStream = ch
	a.streamMu.Unlock()
	a.mu.Unlock()

	if systemPrompt != "" {
		prompt = systemPrompt + "\n\n" + prompt
	}
	req := map[string]any{
		"id":      strconv.Itoa(id),
		"type":    "prompt",
		"message": prompt,
	}
	if err := a.pm.SendJSON(req); err != nil {
		a.streamMu.Lock()
		if a.activeStream == ch {
			close(ch)
			a.activeStream = nil
		}
		a.streamMu.Unlock()
		return nil, fmt.Errorf("send request: %w", err)
	}

	return ch, nil
}

func (a *PiAdapter) LoadSession(acpSessionID string, history []acp.Message) error {
	if err := a.ensureInit(); err != nil {
		return err
	}
	a.mu.Lock()
	currentSessionID := a.sessionID
	a.mu.Unlock()
	if currentSessionID == acpSessionID {
		return nil
	}

	// If the session id looks like a file path, ask pi to switch to it.
	// Otherwise just store the id locally; pi --no-session does not persist state.
	if len(acpSessionID) > 0 && (acpSessionID[0] == '/' || len(acpSessionID) > 5 && acpSessionID[len(acpSessionID)-5:] == ".jsonl") {
		if err := a.sendRequest("switch_session", map[string]any{"sessionPath": acpSessionID}); err != nil {
			return fmt.Errorf("pi switch_session: %w", err)
		}
	}
	a.mu.Lock()
	a.sessionID = acpSessionID
	a.mu.Unlock()
	log.Printf("[pi] session loaded: %s", acpSessionID)
	return nil
}

func (a *PiAdapter) CurrentSessionID() string {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.sessionID
}

func (a *PiAdapter) Info() acp.AgentInfo { return a.info }

func (a *PiAdapter) IsRunning() bool { return a.pm.IsRunning() }
func (a *PiAdapter) Stop() error     { return a.pm.Stop() }

func (a *PiAdapter) SetCWD(cwd string) {
	// Pi does not use cwd parameter in its RPC protocol
}

func (a *PiAdapter) dispatchLoop(pm *acp.ProcessManager) {
	defer func() {
		a.dispatchMu.Lock()
		a.dispatchRunning = false
		a.dispatchMu.Unlock()
	}()
	events := pm.Events()
	done := pm.Done()
	for {
		select {
		case raw, ok := <-events:
			if !ok {
				return
			}
			var msg struct {
				Type string `json:"type"`
				ID   string `json:"id"`
				// response fields
				Command string `json:"command"`
				Success bool   `json:"success"`
				Error   string `json:"error"`
				// event fields
				AssistantMessageEvent struct {
					Type  string `json:"type"`
					Delta string `json:"delta"`
				} `json:"assistantMessageEvent"`
			}

			if err := json.Unmarshal(raw, &msg); err != nil {
				continue
			}

			// PI RPC responses are correlated by id and routed to pending waiters.
			if msg.Type == "response" && msg.ID != "" {
				a.mu.Lock()
				key := fmt.Sprintf("resp-%s", msg.ID)
				waiter, ok := a.pending[key]
				if ok {
					if !msg.Success {
						waiter <- acp.StreamEvent{Type: "error", Error: fmt.Errorf("%s", msg.Error)}
					} else {
						waiter <- acp.StreamEvent{Type: "text_delta", Content: ""}
					}
					close(waiter)
					delete(a.pending, key)
				}
				a.mu.Unlock()
				continue
			}

			switch msg.Type {
			case "message_update":
				if msg.AssistantMessageEvent.Type == "text_delta" {
					text, ok := sanitizeACPText(msg.AssistantMessageEvent.Delta)
					if !ok {
						continue
					}
					a.streamMu.RLock()
					active := a.activeStream
					a.streamMu.RUnlock()
					if active == nil {
						continue
					}
					select {
					case active <- acp.StreamEvent{Type: "text_delta", Content: text}:
					default:
					}
				}
			case "agent_end":
				a.streamMu.RLock()
				active := a.activeStream
				a.streamMu.RUnlock()
				if active == nil {
					continue
				}
				select {
				case active <- acp.StreamEvent{Type: "done"}:
				default:
				}
				a.streamMu.Lock()
				if a.activeStream == active {
					close(a.activeStream)
					a.activeStream = nil
				}
				a.streamMu.Unlock()
			}

		case <-done:
			return
		}
	}
}

func (a *PiAdapter) sendRequest(command string, params map[string]any) error {
	a.mu.Lock()
	a.reqID++
	reqID := strconv.Itoa(a.reqID)
	key := fmt.Sprintf("resp-%s", reqID)
	ch := make(chan acp.StreamEvent, 2)
	a.pending[key] = ch
	a.mu.Unlock()

	req := map[string]any{
		"id":     reqID,
		"type":   command,
		"params": params,
	}
	if err := a.pm.SendJSON(req); err != nil {
		a.mu.Lock()
		delete(a.pending, key)
		a.mu.Unlock()
		return err
	}

	select {
	case evt, ok := <-ch:
		if !ok {
			return fmt.Errorf("channel closed for %s", command)
		}
		if evt.IsError() {
			return evt.Error
		}
		return nil
	case <-time.After(15 * time.Second):
		a.mu.Lock()
		delete(a.pending, key)
		a.mu.Unlock()
		return fmt.Errorf("timeout waiting for %s", command)
	}
}

var _ acp.ACPAdapter = (*PiAdapter)(nil)

// IsPiCliInstalled reports whether the Pi CLI is available on PATH.
func IsPiCliInstalled() bool {
	_, err := exec.LookPath("pi")
	return err == nil
}
