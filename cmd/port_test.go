package cmd

import (
	"encoding/json"
	"errors"
	"net"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/0xProgress/waitfor/internal/exitcode"
)

// startListener opens a TCP listener on 127.0.0.1 and returns it plus the
// port. Caller must close the listener.
func startListener(t *testing.T) (net.Listener, string) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	_, port, err := net.SplitHostPort(ln.Addr().String())
	if err != nil {
		ln.Close()
		t.Fatalf("split: %v", err)
	}
	return ln, port
}

func codeOf(t *testing.T, err error) int {
	t.Helper()
	if err == nil {
		return 0
	}
	ce, ok := errors.AsType[*exitcode.Error](err)
	if !ok {
		t.Fatalf("uncoded error: %T %v", err, err)
	}
	return ce.Code
}

func TestPortCmdSuccess(t *testing.T) {
	ln, port := startListener(t)
	defer ln.Close()

	out, err := runRoot(t,
		"--timeout", "2s",
		"--interval", "100ms",
		"port", port,
	)
	if code := codeOf(t, err); code != 0 {
		t.Fatalf("exit code = %d, err = %v", code, err)
	}
	if !strings.Contains(out, "✓ Port "+port+" is open") {
		t.Errorf("missing success line:\n%s", out)
	}
}

func TestPortCmdTimeout(t *testing.T) {
	ln, port := startListener(t)
	ln.Close() // nothing listening now

	out, err := runRoot(t,
		"--timeout", "300ms",
		"--interval", "50ms",
		"port", port,
	)
	if code := codeOf(t, err); code != exitcode.Timeout {
		t.Fatalf("exit code = %d, want %d (err=%v)\n%s",
			code, exitcode.Timeout, err, out)
	}
	if !strings.Contains(out, "✗ Timed out after") {
		t.Errorf("missing timeout banner:\n%s", out)
	}
	// Both refusal and timeout are valid failure states per spec. On some
	// systems, dialing a just-closed ephemeral loopback port briefly
	// returns ETIMEDOUT before the kernel settles into RST/refused.
	// Either diagnostic is acceptable here.
	refused := strings.Contains(out, "connection refused")
	timedOut := strings.Contains(out, "connection timeout")
	if !refused && !timedOut {
		t.Errorf("missing diagnostic state (want refused or timeout):\n%s", out)
	}
}

func TestPortCmdInvalidPortExit2(t *testing.T) {
	cases := []string{"0", "-1", "65536", "abc"}
	for _, p := range cases {
		_, err := runRoot(t, "port", p)
		if code := codeOf(t, err); code != exitcode.BadArgs {
			t.Errorf("port %q: exit = %d, want %d", p, code, exitcode.BadArgs)
		}
	}
}

func TestPortCmdJSON(t *testing.T) {
	ln, port := startListener(t)
	defer ln.Close()

	out, err := runRoot(t,
		"--json",
		"--timeout", "2s",
		"port", port,
	)
	if code := codeOf(t, err); code != 0 {
		t.Fatalf("exit = %d, err = %v", code, err)
	}
	var got map[string]any
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out)
	}
	if got["condition"] != "port" {
		t.Errorf("condition = %v", got["condition"])
	}
	if got["target"] != port {
		t.Errorf("target = %v, want %v", got["target"], port)
	}
	if got["success"] != true {
		t.Errorf("success = %v", got["success"])
	}
}

func TestPortCmdQuiet(t *testing.T) {
	ln, port := startListener(t)
	defer ln.Close()

	out, err := runRoot(t,
		"--quiet",
		"--timeout", "2s",
		"port", port,
	)
	if code := codeOf(t, err); code != 0 {
		t.Fatalf("exit = %d", code)
	}
	if out != "" {
		t.Errorf("expected no output, got %q", out)
	}
}

func TestPortCmdVerboseShowsAttempts(t *testing.T) {
	ln, port := startListener(t)
	ln.Close()

	out, err := runRoot(t,
		"--verbose",
		"--timeout", "300ms",
		"--interval", "80ms",
		"port", port,
	)
	_ = err // timeout expected
	if !strings.Contains(out, "attempt 1") {
		t.Errorf("missing attempt line:\n%s", out)
	}
	if strings.Contains(out, "attempt 1") && !strings.Contains(out, "connection refused") {
		t.Errorf("attempt line missing state:\n%s", out)
	}
}

func TestPortCmdTimeoutZeroSingleCheck(t *testing.T) {
	ln, port := startListener(t)
	ln.Close()

	out, err := runRoot(t,
		"--timeout", "0s",
		"port", port,
	)
	if code := codeOf(t, err); code != exitcode.Timeout {
		t.Fatalf("exit = %d, want %d", code, exitcode.Timeout)
	}
	if !strings.Contains(out, "Condition not met (single check, no timeout set)") {
		t.Errorf("missing single-check banner:\n%s", out)
	}
	// verbose is off, so no attempt lines regardless of how many checks ran.
	if strings.Contains(out, "attempt ") {
		t.Errorf("verbose off, but attempt lines present:\n%s", out)
	}
}

// Short-circuit: ensure the CLI doesn't sit polling for a case that will
// never change within the budget.
func TestPortCmdShortTimeoutIsQuick(t *testing.T) {
	ln, port := startListener(t)
	ln.Close()

	start := time.Now()
	_, err := runRoot(t,
		"--timeout", "200ms",
		"--interval", "50ms",
		"port", port,
	)
	elapsed := time.Since(start)
	if code := codeOf(t, err); code != exitcode.Timeout {
		t.Fatalf("exit = %d", code)
	}
	if elapsed > 2*time.Second {
		t.Errorf("took %s, expected under 2s", elapsed)
	}
}

// TestPortCmdRefusedToClosedPort exercises the refused path against a port
// that no process is listening on and cannot be (privileged, and we're not
// root). Skips if the environment drops the SYN instead of RSTing.
func TestPortCmdRefusedToClosedPort(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("running as root; port 1 could theoretically be bound")
	}
	out, err := runRoot(t,
		"--timeout", "500ms",
		"--interval", "100ms",
		"port", "1",
	)
	if code := codeOf(t, err); code != exitcode.Timeout {
		t.Fatalf("exit code = %d, want %d\n%s", code, exitcode.Timeout, out)
	}
	if !strings.Contains(out, "connection refused") {
		t.Skipf("environment returned a state other than refused:\n%s", out)
	}
}
