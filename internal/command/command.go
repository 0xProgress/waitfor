// Package command implements waitfor conditions that run a shell command
// and wait for it to exit with the expected code.
//
// The command string is user-supplied and is passed through to the shell
// unmodified. It runs as the current user only; waitfor never elevates.
package command

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strings"
)

const maxStderrDisplay = 200

// CommandCondition waits for a shell command to exit with the expected code.
type CommandCondition struct {
	cmdStr   string
	wantCode int
	lastErr  string
}

// CommandOptions configures NewCommandCondition.
type CommandOptions struct {
	// ExitCode is the expected exit code. Defaults to 0.
	ExitCode int
}

// NewCommandCondition validates the command string and returns a condition.
func NewCommandCondition(cmdStr string, opts CommandOptions) (*CommandCondition, error) {
	if strings.TrimSpace(cmdStr) == "" {
		return nil, fmt.Errorf("command: empty command")
	}
	return &CommandCondition{
		cmdStr:   cmdStr,
		wantCode: opts.ExitCode,
	}, nil
}

func (c *CommandCondition) Kind() string   { return "command" }
func (c *CommandCondition) Target() string { return c.cmdStr }
func (c *CommandCondition) Describe() string {
	return fmt.Sprintf("command %q", c.cmdStr)
}
func (c *CommandCondition) SuccessMessage() string {
	return fmt.Sprintf("command %q exited with code %d", c.cmdStr, c.wantCode)
}

// LastDetail implements poller.Detailer.
func (c *CommandCondition) LastDetail() (string, string) {
	return "Stderr", c.lastErr
}

// Check runs the command once and evaluates the exit code.
//
// Failure states produced:
//   - "exit code N"            (mismatch with wantCode)
//   - "command failed: <err>"  (couldn't start the shell)
func (c *CommandCondition) Check(ctx context.Context) (string, bool, error) {
	c.lastErr = ""

	cmd := shellCommand(ctx, c.cmdStr)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	cmd.Stdout = io.Discard

	err := cmd.Run()

	code, ok := exitCodeFromError(err)
	if !ok {
		// No exit code available: either the shell couldn't start, or the
		// process was killed by a signal (typically our ctx cancellation).
		if ctx.Err() != nil {
			return "still running (did not exit before deadline)", false, nil
		}
		return "command failed: " + err.Error(), false, nil
	}

	if code == c.wantCode {
		return fmt.Sprintf("exit code %d", code), true, nil
	}

	c.lastErr = truncateStderr(stderr.Bytes())
	return fmt.Sprintf("exit code %d", code), false, nil
}

// exitCodeFromError extracts the exit code from a completed command.
// Returns (0, true) for a clean exit; (code, true) for a non-zero exit;
// (0, false) if the process was killed by a signal or the error isn't an
// ExitError at all (shell couldn't start, etc.).
//
// ExitCode() returns -1 for signal-killed processes, which is how we
// detect "was killed" vs "exited with a code."
func exitCodeFromError(err error) (int, bool) {
	if err == nil {
		return 0, true
	}
	if ee, ok := errors.AsType[*exec.ExitError](err); ok {
		if ee.ExitCode() < 0 {
			return 0, false
		}
		return ee.ExitCode(), true
	}
	return 0, false
}

// truncateStderr collapses whitespace and caps length for display.
func truncateStderr(b []byte) string {
	s := strings.Join(strings.Fields(string(b)), " ")
	if len(s) > maxStderrDisplay {
		s = s[:maxStderrDisplay] + "…"
	}
	return s
}
