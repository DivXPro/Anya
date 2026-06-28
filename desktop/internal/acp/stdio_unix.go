//go:build !windows

package acp

import (
	"context"
	"errors"
	"log"
	"os/exec"
	"syscall"
	"time"
)

func buildCommand(ctx context.Context, command string) (*exec.Cmd, error) {
	return exec.CommandContext(ctx, "sh", "-c", command), nil
}

func configureProcess(cmd *exec.Cmd) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.Setpgid = true
}

func terminateProcess(cmd *exec.Cmd, exited <-chan struct{}) error {
	pid := cmd.Process.Pid
	var lastErr error

	if err := syscall.Kill(-pid, syscall.SIGTERM); err != nil {
		log.Printf("[stdio] SIGTERM error: %v", err)
		if !errors.Is(err, syscall.ESRCH) {
			lastErr = err
		}
	}

	select {
	case <-exited:
		log.Printf("[stdio] process exited gracefully")
	case <-time.After(10 * time.Second):
		log.Printf("[stdio] process didn't exit, sending SIGKILL")
		if err := syscall.Kill(-pid, syscall.SIGKILL); err != nil {
			log.Printf("[stdio] SIGKILL error: %v", err)
			if !errors.Is(err, syscall.ESRCH) {
				lastErr = err
			}
		}
		<-exited
	}
	return lastErr
}
