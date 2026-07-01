//go:build windows

package agentinstall

import (
	"os/exec"
	"strconv"
)

// setProcAttr is a no-op on Windows; process-tree teardown is handled by
// killProcessTree via taskkill, which is more reliable than process groups for
// killing the descendants a `curl | bash`-equivalent installer spawns.
func setProcAttr(cmd *exec.Cmd) {}

// killProcessTree force-kills the process and all of its children using
// taskkill, so an installer that spawned node/npm subprocesses does not linger.
func killProcessTree(cmd *exec.Cmd) error {
	if cmd.Process == nil {
		return nil
	}
	pid := cmd.Process.Pid
	kill := exec.Command("taskkill", "/T", "/F", "/PID", strconv.Itoa(pid))
	if err := kill.Run(); err != nil {
		return cmd.Process.Kill()
	}
	return nil
}
