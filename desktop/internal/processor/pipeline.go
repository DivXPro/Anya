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

	noResponseTimeout time.Duration
	execTimeout       time.Duration

	startTime      time.Time
	lastEventAt    time.Time
	onExecTimeout  func()
}

type PipelineResult int

const (
	ResultComplete          PipelineResult = iota
	ResultNoResponseTimeout
	ResultExecTimeout
)

func NewPipeline(events <-chan acp.StreamEvent) *Pipeline {
	now := time.Now()
	return &Pipeline{
		events:            events,
		noResponseTimeout: 30 * time.Second,
		execTimeout:       300 * time.Second,
		startTime:         now,
		lastEventAt:       now,
	}
}

func (p *Pipeline) SetExecTimeoutCallback(cb func()) {
	p.onExecTimeout = cb
}

func (p *Pipeline) Process() (*Response, PipelineResult, error) {
	noResponseTimer := time.NewTimer(p.noResponseTimeout)
	execTimer := time.NewTimer(p.execTimeout)
	defer noResponseTimer.Stop()
	defer execTimer.Stop()

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

		case <-execTimer.C:
			if p.onExecTimeout != nil {
				p.onExecTimeout()
			}
			execTimer.Reset(p.execTimeout)
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
