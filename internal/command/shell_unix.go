//go:build unix

package command

import (
	"context"
	"os/exec"
	"syscall"
)

// shellCommand returns an *exec.Cmd that runs cmdStr through `sh -c`, so
// pipes, redirection, and command substitution behave as the user expects.
//
// The child is placed in its own process group so cancellation kills the
// whole group — sh and any descendants. Without this, exec's default
// Cancel kills only the direct child, and grandchildren (e.g. `sleep`
// spawned by `sh -c "sleep 5"`) survive.
func shellCommand(ctx context.Context, cmdStr string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, "sh", "-c", cmdStr)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		if cmd.Process == nil {
			return nil
		}
		// Negative PID = the whole process group.
		return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	}
	return cmd
}
