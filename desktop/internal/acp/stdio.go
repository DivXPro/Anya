package acp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os/exec"
	"strings"
	"sync"
)

type Framing int

const (
	LSPFraming Framing = iota
	NDJSONFraming
)

type ProcessManager struct {
	mu      sync.Mutex
	writeMu sync.Mutex
	cmd     *exec.Cmd
	stdin   io.WriteCloser
	stdout  io.ReadCloser
	events  chan []byte
	done    chan struct{}
	ctx     context.Context
	cancel  context.CancelFunc
	command string
	framing Framing
	running bool
	exited  chan struct{}
}

func NewProcessManager(command string) *ProcessManager {
	return NewProcessManagerWithFraming(command, LSPFraming)
}

func NewProcessManagerWithFraming(command string, framing Framing) *ProcessManager {
	return &ProcessManager{
		command: command,
		framing: framing,
		events:  make(chan []byte, 256),
		done:    make(chan struct{}),
	}
}

func (pm *ProcessManager) Start() error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	if pm.running {
		return nil
	}
	if strings.TrimSpace(pm.command) == "" {
		return fmt.Errorf("command is empty")
	}

	pm.ctx, pm.cancel = context.WithCancel(context.Background())
	cmd, err := buildCommand(pm.ctx, pm.command)
	if err != nil {
		pm.cancel()
		return fmt.Errorf("build command: %w", err)
	}
	pm.cmd = cmd
	configureProcess(pm.cmd)
	pm.exited = make(chan struct{})
	pm.done = make(chan struct{})

	pm.stdin, err = pm.cmd.StdinPipe()
	if err != nil {
		pm.cancel()
		return fmt.Errorf("stdin pipe: %w", err)
	}
	pm.stdout, err = pm.cmd.StdoutPipe()
	if err != nil {
		pm.cancel()
		return fmt.Errorf("stdout pipe: %w", err)
	}
	pm.cmd.Stderr = log.Writer()

	if err := pm.cmd.Start(); err != nil {
		pm.cancel()
		return fmt.Errorf("start process: %w", err)
	}
	pm.running = true

	stdout := pm.stdout
	ctx := pm.ctx
	exited := pm.exited
	done := pm.done
	switch pm.framing {
	case NDJSONFraming:
		go pm.readLoopNDJSON(stdout, ctx, done)
	default:
		go pm.readLoopLSP(stdout, ctx, done)
	}
	go pm.waitLoop(pm.cmd, exited, ctx)

	log.Printf("[stdio] process started: %s (pid=%d)", pm.command, pm.cmd.Process.Pid)
	return nil
}

func (pm *ProcessManager) readLoopLSP(reader io.ReadCloser, ctx context.Context, done chan struct{}) {
	defer close(done)
	br := bufio.NewReader(reader)
	for {
		headerLine, err := br.ReadString('\n')
		if err != nil {
			if err != io.EOF && ctx.Err() == nil {
				log.Printf("[stdio] read header error: %v", err)
			}
			return
		}
		headerLine = strings.TrimSpace(headerLine)
		if headerLine == "" {
			continue
		}

		var contentLen int
		if _, err := fmt.Sscanf(headerLine, "Content-Length: %d", &contentLen); err != nil {
			log.Printf("[stdio] malformed header: %s", headerLine)
			continue
		}

		if _, err := br.ReadString('\n'); err != nil {
			log.Printf("[stdio] read separator error: %v", err)
			return
		}

		body := make([]byte, contentLen)
		if _, err := io.ReadFull(br, body); err != nil {
			log.Printf("[stdio] read body error: %v", err)
			return
		}

		select {
		case pm.events <- body:
		case <-ctx.Done():
			return
		}
	}
}

func (pm *ProcessManager) readLoopNDJSON(reader io.ReadCloser, ctx context.Context, done chan struct{}) {
	defer close(done)
	br := bufio.NewReader(reader)
	for {
		line, err := br.ReadBytes('\n')
		if err != nil {
			if err != io.EOF && ctx.Err() == nil {
				log.Printf("[stdio] read line error: %v", err)
			}
			return
		}
		line = bytes.TrimSpace(line)
		if len(line) == 0 {
			continue
		}

		select {
		case pm.events <- line:
		case <-ctx.Done():
			return
		}
	}
}

func (pm *ProcessManager) waitLoop(cmd *exec.Cmd, exited chan struct{}, ctx context.Context) {
	err := cmd.Wait()
	pm.mu.Lock()
	pm.running = false
	pm.mu.Unlock()
	if err != nil && ctx.Err() == nil {
		log.Printf("[stdio] process exited with error: %v", err)
	}
	if exited != nil {
		close(exited)
	}
}

func (pm *ProcessManager) Stop() error {
	pm.mu.Lock()
	if !pm.running || pm.cmd == nil || pm.cmd.Process == nil {
		pm.mu.Unlock()
		return nil
	}
	cmd := pm.cmd
	exited := pm.exited
	cancel := pm.cancel
	pm.mu.Unlock()

	log.Printf("[stdio] stopping process (pid=%d)...", cmd.Process.Pid)

	err := terminateProcess(cmd, exited)
	cancel()
	return err
}

func (pm *ProcessManager) IsRunning() bool {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	return pm.running
}

func (pm *ProcessManager) Send(data []byte) error {
	pm.mu.Lock()
	if !pm.running {
		pm.mu.Unlock()
		return fmt.Errorf("process not running")
	}
	stdin := pm.stdin
	framing := pm.framing
	pm.mu.Unlock()

	var frame []byte
	switch framing {
	case NDJSONFraming:
		frame = append(data, '\n')
	default:
		header := fmt.Sprintf("Content-Length: %d\r\n\r\n", len(data))
		frame = append([]byte(header), data...)
	}

	// Serialize concurrent writers with a dedicated mutex instead of pm.mu, so a
	// write that blocks (child stopped draining stdin, pipe buffer full) cannot
	// wedge Stop()/IsRunning() — which need pm.mu to terminate the child and
	// unblock the write.
	pm.writeMu.Lock()
	defer pm.writeMu.Unlock()
	if _, err := stdin.Write(frame); err != nil {
		return fmt.Errorf("write to stdin: %w", err)
	}
	return nil
}

func (pm *ProcessManager) Events() <-chan []byte {
	return pm.events
}

func (pm *ProcessManager) Done() <-chan struct{} {
	return pm.done
}

func (pm *ProcessManager) SendJSON(v interface{}) error {
	data, err := json.Marshal(v)
	if err != nil {
		return fmt.Errorf("marshal json: %w", err)
	}
	return pm.Send(data)
}
