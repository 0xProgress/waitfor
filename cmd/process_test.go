package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/0xProgress/waitfor/internal/exitcode"
)

// selfProcessName returns a name that identifies the current test binary,
// trimmed to Linux's 15-char comm limit. Used so the success case has a
// target we know exists.
func selfProcessName(t *testing.T) string {
	t.Helper()
	exe, err := os.Executable()
	if err != nil {
		t.Skipf("cannot determine own executable: %v", err)
	}
	base := filepath.Base(exe)
	if base == "" {
		t.Skip("empty executable basename")
	}
	if len(base) > 15 {
		base = base[:15]
	}
	return base
}

func TestProcessCmdFindsSelf(t *testing.T) {
	name := selfProcessName(t)
	out, err := runRoot(t, "--timeout", "3s", "process", name)
	if code := codeOf(t, err); code != 0 {
		t.Fatalf("exit = %d, err = %v\n%s", code, err, out)
	}
	if !strings.Contains(out, "✓") {
		t.Errorf("missing success line:\n%s", out)
	}
}

func TestProcessCmdTimeout(t *testing.T) {
	out, err := runRoot(t,
		"--timeout", "300ms",
		"--interval", "80ms",
		"process", "definitely-not-a-real-process-xyz123",
	)
	if code := codeOf(t, err); code != exitcode.Timeout {
		t.Fatalf("exit = %d, want %d\n%s", code, exitcode.Timeout, out)
	}
	for _, want := range []string{
		"✗ Timed out after",
		"no matching process found",
		"Tip:",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
}

func TestProcessCmdNumericPIDExit2(t *testing.T) {
	_, err := runRoot(t, "process", "1234")
	if code := codeOf(t, err); code != exitcode.BadArgs {
		t.Errorf("exit = %d, want %d", code, exitcode.BadArgs)
	}
}

func TestProcessCmdUnknownUserExit2(t *testing.T) {
	_, err := runRoot(t, "process", "bash", "--user", "definitely-no-such-user-xyz")
	if code := codeOf(t, err); code != exitcode.BadArgs {
		t.Errorf("exit = %d, want %d", code, exitcode.BadArgs)
	}
}

func TestProcessCmdJSON(t *testing.T) {
	name := selfProcessName(t)
	out, err := runRoot(t, "--json", "--timeout", "3s", "process", name)
	if code := codeOf(t, err); code != 0 {
		t.Fatalf("exit = %d, err = %v\n%s", code, err, out)
	}
	for _, want := range []string{
		`"condition": "process"`,
		`"target": "` + name + `"`,
		`"success": true`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
}
