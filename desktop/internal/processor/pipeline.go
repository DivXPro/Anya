package processor

import (
	"fmt"
	"strings"
	"time"

	"desktop/internal/acp"
)

type Response struct {
	Content string
	Summary string
}

type Pipeline struct {
	events  <-chan acp.StreamEvent
	content strings.Builder

	// noResponseTimeout is an idle timeout: it resets on every event, so it
	// fires only after the agent has been completely silent for this long.
	noResponseTimeout time.Duration
	// heartbeatInterval drives a periodic "still working" callback so the
	// device keeps receiving a liveness signal during long turns.
	heartbeatInterval time.Duration
	// maxTurnTimeout is a hard wall-clock cap on a single turn. It is NOT reset
	// on events, so even a backend that streams forever is bounded.
	maxTurnTimeout time.Duration

	startTime   time.Time
	lastEventAt time.Time
	onHeartbeat func()
}

type PipelineResult int

const (
	ResultComplete PipelineResult = iota
	ResultNoResponseTimeout
	ResultExecTimeout
)

func NewPipeline(events <-chan acp.StreamEvent) *Pipeline {
	now := time.Now()
	return &Pipeline{
		events:            events,
		noResponseTimeout: 30 * time.Second,
		heartbeatInterval: 12 * time.Second,
		maxTurnTimeout:    10 * time.Minute,
		startTime:         now,
		lastEventAt:       now,
	}
}

// SetHeartbeatCallback registers a callback invoked periodically
// (every heartbeatInterval) while a turn is still being processed.
func (p *Pipeline) SetHeartbeatCallback(cb func()) {
	p.onHeartbeat = cb
}

func (p *Pipeline) Process() (*Response, PipelineResult, error) {
	noResponseTimer := time.NewTimer(p.noResponseTimeout)
	heartbeatTimer := time.NewTimer(p.heartbeatInterval)
	maxTurnTimer := time.NewTimer(p.maxTurnTimeout)
	defer noResponseTimer.Stop()
	defer heartbeatTimer.Stop()
	defer maxTurnTimer.Stop()

	for {
		select {
		case evt, ok := <-p.events:
			if !ok {
				return nil, ResultComplete, nil
			}

			if !noResponseTimer.Stop() {
				select {
				case <-noResponseTimer.C:
				default:
				}
			}
			noResponseTimer.Reset(p.noResponseTimeout)
			p.lastEventAt = time.Now()

			switch {
			case evt.IsError():
				return nil, ResultComplete, evt.Error

			case evt.IsContent():
				p.content.WriteString(evt.Content)

			case evt.IsSkippable():
				continue

			case evt.IsDone():
				content := p.content.String()
				summary, err := Summarize(content)
				if err != nil {
					summary = truncate(content, 50)
				}
				return &Response{Content: content, Summary: summary}, ResultComplete, nil
			}

		case <-noResponseTimer.C:
			return nil, ResultNoResponseTimeout, fmt.Errorf("no response from agent within %v", p.noResponseTimeout)

		case <-maxTurnTimer.C:
			// Hard cap: a turn that keeps streaming without ever finishing is
			// still bounded so the caller can send a terminal message.
			return nil, ResultExecTimeout, fmt.Errorf("turn exceeded max duration %v", p.maxTurnTimeout)

		case <-heartbeatTimer.C:
			if p.onHeartbeat != nil {
				p.onHeartbeat()
			}
			heartbeatTimer.Reset(p.heartbeatInterval)
		}
	}
}

func truncate(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen-3]) + "..."
}
