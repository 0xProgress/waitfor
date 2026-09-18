//go:build windows

package command

import (
	"context"
	"os/exec"
)

// shellCommand returns an *exec.Cmd that runs cmdStr through `cmd /C`.
func shellCommand(ctx context.Context, cmdStr string) *exec.Cmd {
	return exec.CommandContext(ctx, "cmd", "/C", cmdStr)
}
