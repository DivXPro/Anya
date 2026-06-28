//go:build windows

package acp

import (
	"context"
	"errors"
	"log"
	"os/exec"
	"strings"
	"syscall"
	"time"
)

func splitCommandWindows(command string) ([]string, error) {
	var args []string
	var current strings.Builder
	inQuotes := false

	for _, r := range command {
		switch r {
		case '"':
			inQuotes = !inQuotes
		case ' ', '\t', '\n', '\r':
			if inQuotes {
				current.WriteRune(r)
			} else if current.Len() > 0 {
				args = append(args, current.String())
				current.Reset()
			}
		default:
			current.WriteRune(r)
		}
	}
	if current.Len() > 0 {
		args = append(args, current.String())
	}
	if len(args) == 0 {
		return nil, errors.New("empty command")
	}
	return args, nil
}

func buildCommand(ctx context.Context, command string) (*exec.Cmd, error) {
	args, err := splitCommandWindows(command)
	if err != nil {
		return nil, err
	}
	return exec.CommandContext(ctx, args[0], args[1:]...), nil
}

func configureProcess(cmd *exec.Cmd) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.HideWindow = true
}

func terminateProcess(cmd *exec.Cmd, exited <-chan struct{}) error {
	var lastErr error
	if err := cmd.Process.Kill(); err != nil {
		log.Printf("[stdio] Kill error: %v", err)
		lastErr = err
	}

	select {
	case <-exited:
		log.Printf("[stdio] process exited")
	case <-time.After(10 * time.Second):
		err := errors.New("process did not exit within timeout")
		log.Printf("[stdio] %v", err)
		lastErr = err
	}
	return lastErr
}
