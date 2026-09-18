package cmd

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/0xProgress/waitfor/internal/exitcode"
)

func TestCommandCmdSuccess(t *testing.T) {
	out, err := runRoot(t, "--timeout", "2s", "command", "true")
	if code := codeOf(t, err); code != 0 {
		t.Fatalf("exit = %d, err = %v\n%s", code, err, out)
	}
	if !strings.Contains(out, "✓") {
		t.Errorf("missing success line:\n%s", out)
	}
}

func TestCommandCmdTimeoutWithStderr(t *testing.T) {
	out, err := runRoot(t,
		"--timeout", "300ms",
		"--interval", "80ms",
		"command", `echo "boom happened" >&2; exit 1`,
	)
	if code := codeOf(t, err); code != exitcode.Timeout {
		t.Fatalf("exit = %d, want %d\n%s", code, exitcode.Timeout, out)
	}
	for _, want := range []string{
		"✗ Timed out after",
		`command "echo`,
		"exit code 1",
		"Stderr:     boom happened",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
}

func TestCommandCmdCustomExitCode(t *testing.T) {
	out, err := runRoot(t, "--timeout", "2s", "command", "exit 42", "--exit-code", "42")
	if code := codeOf(t, err); code != 0 {
		t.Fatalf("exit = %d, err = %v\n%s", code, err, out)
	}
}

func TestCommandCmdEmptyExit2(t *testing.T) {
	_, err := runRoot(t, "command", "   ")
	if code := codeOf(t, err); code != exitcode.BadArgs {
		t.Errorf("exit = %d, want %d", code, exitcode.BadArgs)
	}
}

func TestCommandCmdPipe(t *testing.T) {
	out, err := runRoot(t,
		"--timeout", "2s",
		"command", "echo hello | grep hello",
	)
	if code := codeOf(t, err); code != 0 {
		t.Fatalf("exit = %d, err = %v\n%s", code, err, out)
	}
}

func TestCommandCmdJSON(t *testing.T) {
	out, err := runRoot(t,
		"--json",
		"--timeout", "300ms",
		"--interval", "80ms",
		"command", `echo "nope" >&2; exit 2`,
	)
	if code := codeOf(t, err); code != exitcode.Timeout {
		t.Fatalf("exit = %d", code)
	}
	var got map[string]any
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out)
	}
	if got["condition"] != "command" {
		t.Errorf("condition = %v", got["condition"])
	}
	if got["detail_label"] != "Stderr" {
		t.Errorf("detail_label = %v", got["detail_label"])
	}
	if got["detail_value"] != "nope" {
		t.Errorf("detail_value = %v", got["detail_value"])
	}
	if got["last_state"] != "exit code 2" {
		t.Errorf("last_state = %v", got["last_state"])
	}
}

func TestCommandCmdQuiet(t *testing.T) {
	out, err := runRoot(t, "--quiet", "--timeout", "2s", "command", "true")
	if code := codeOf(t, err); code != 0 {
		t.Fatalf("exit = %d", code)
	}
	if out != "" {
		t.Errorf("expected empty output, got %q", out)
	}
}
