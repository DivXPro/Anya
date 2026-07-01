//go:build !windows

package agentinstall

import (
	"bufio"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

// TestKillProcessTree_KillsGrandchild verifies the core orphan fix: a child that
// spawns its own subprocess (as `curl | bash` or an npm postinstall would) is
// torn down entirely when the process group is killed, leaving nothing behind.
func TestKillProcessTree_KillsGrandchild(t *testing.T) {
	// Parent shell spawns a background sleep (the "grandchild") and prints its
	// PID, then waits — mirroring an installer that forks a long-running child.
	cmd := exec.Command("/bin/sh", "-c", "sleep 30 & echo $!; wait")
	setProcAttr(cmd)
	out, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}

	line, err := bufio.NewReader(out).ReadString('\n')
	if err != nil {
		t.Fatalf("read grandchild pid: %v", err)
	}
	childPid, err := strconv.Atoi(strings.TrimSpace(line))
	if err != nil {
		t.Fatalf("parse grandchild pid %q: %v", line, err)
	}

	if err := killProcessTree(cmd); err != nil {
		t.Fatalf("kill process tree: %v", err)
	}
	_ = cmd.Wait()

	// Signal 0 probes liveness; once the grandchild is killed and reaped it
	// returns ESRCH. Poll briefly to let the kernel deliver SIGKILL.
	gone := false
	for i := 0; i < 100; i++ {
		if err := syscall.Kill(childPid, 0); err != nil {
			gone = true
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if !gone {
		t.Fatalf("expected grandchild pid %d to be killed with the group", childPid)
	}
}
