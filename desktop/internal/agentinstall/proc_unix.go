//go:build !windows

package agentinstall

import (
	"os/exec"
	"syscall"
)

// setProcAttr puts the install command in its own process group so that the
// whole tree (e.g. bash -> curl -> node) can be killed together. exec's default
// Cancel only signals the direct child, which would leave a `curl | bash`
// pipeline or an npm postinstall orphaned and immune to the context timeout.
func setProcAttr(cmd *exec.Cmd) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.Setpgid = true
}

// killProcessTree kills the entire process group started by setProcAttr.
// Signalling the negative pid targets the group leader and all descendants.
func killProcessTree(cmd *exec.Cmd) error {
	if cmd.Process == nil {
		return nil
	}
	pgid := cmd.Process.Pid
	// A negative pid addresses the whole process group.
	if err := syscall.Kill(-pgid, syscall.SIGKILL); err != nil {
		// Fall back to killing just the leader if the group send failed.
		return cmd.Process.Kill()
	}
	return nil
}
