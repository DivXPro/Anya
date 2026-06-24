package acp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"strings"
	"sync"
	"syscall"
	"time"
)

type ProcessManager struct {
	mu      sync.Mutex
	cmd     *exec.Cmd
	stdin   io.WriteCloser
	stdout  io.ReadCloser
	events  chan []byte
	done    chan struct{}
	ctx     context.Context
	cancel  context.CancelFunc
	command string
	running bool
	exited  chan struct{}
}

func NewProcessManager(command string) *ProcessManager {
	return &ProcessManager{
		command: command,
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

	pm.ctx, pm.cancel = context.WithCancel(context.Background())
	pm.cmd = exec.CommandContext(pm.ctx, "sh", "-c", pm.command)
	pm.cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	pm.exited = make(chan struct{})
	pm.done = make(chan struct{})

	var err error
	pm.stdin, err = pm.cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("stdin pipe: %w", err)
	}
	pm.stdout, err = pm.cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("stdout pipe: %w", err)
	}
	pm.cmd.Stderr = os.Stderr

	if err := pm.cmd.Start(); err != nil {
		return fmt.Errorf("start process: %w", err)
	}
	pm.running = true

	go pm.readLoop()
	go pm.waitLoop()

	log.Printf("[stdio] process started: %s (pid=%d)", pm.command, pm.cmd.Process.Pid)
	return nil
}

func (pm *ProcessManager) readLoop() {
	defer close(pm.done)
	reader := bufio.NewReader(pm.stdout)
	for {
		headerLine, err := reader.ReadString('\n')
		if err != nil {
			if err != io.EOF && pm.ctx.Err() == nil {
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

		reader.ReadString('\n')

		body := make([]byte, contentLen)
		if _, err := io.ReadFull(reader, body); err != nil {
			log.Printf("[stdio] read body error: %v", err)
			return
		}

		select {
		case pm.events <- body:
		case <-pm.ctx.Done():
			return
		}
	}
}

func (pm *ProcessManager) waitLoop() {
	err := pm.cmd.Wait()
	pm.mu.Lock()
	pm.running = false
	pm.mu.Unlock()
	if err != nil && pm.ctx.Err() == nil {
		log.Printf("[stdio] process exited with error: %v", err)
	}
	if pm.exited != nil {
		close(pm.exited)
	}
}

func (pm *ProcessManager) Stop() error {
	pm.mu.Lock()
	if !pm.running || pm.cmd == nil {
		pm.mu.Unlock()
		return nil
	}
	pid := pm.cmd.Process.Pid
	exited := pm.exited
	pm.mu.Unlock()

	log.Printf("[stdio] stopping process (pid=%d)...", pid)

	if err := syscall.Kill(-pid, syscall.SIGTERM); err != nil {
		log.Printf("[stdio] SIGTERM error: %v", err)
	}

	select {
	case <-exited:
		log.Printf("[stdio] process exited gracefully")
	case <-time.After(10 * time.Second):
		log.Printf("[stdio] process didn't exit, sending SIGKILL")
		syscall.Kill(-pid, syscall.SIGKILL)
		<-exited
	}

	pm.cancel()
	return nil
}

func (pm *ProcessManager) IsRunning() bool {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	return pm.running
}

func (pm *ProcessManager) Send(data []byte) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	if !pm.running {
		return fmt.Errorf("process not running")
	}
	header := fmt.Sprintf("Content-Length: %d\r\n\r\n", len(data))
	if _, err := pm.stdin.Write([]byte(header)); err != nil {
		return err
	}
	_, err := pm.stdin.Write(data)
	return err
}

func (pm *ProcessManager) Events() <-chan []byte {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	return pm.events
}

func (pm *ProcessManager) Done() <-chan struct{} {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	return pm.done
}

func (pm *ProcessManager) SendJSON(v interface{}) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return pm.Send(data)
}
