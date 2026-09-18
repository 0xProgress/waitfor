package cmd

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/0xProgress/waitfor/internal/exitcode"
	"github.com/0xProgress/waitfor/internal/poller"
)

// T2 — --timeout 0s = one check, no loop.
func TestEdgeTimeoutZeroSingleCheck(t *testing.T) {
	ln, port := startListener(t)
	ln.Close()

	out, err := runRoot(t,
		"--verbose",
		"--timeout", "0s",
		"port", port,
	)
	if code := codeOf(t, err); code != exitcode.Timeout {
		t.Fatalf("exit = %d, want %d\n%s", code, exitcode.Timeout, out)
	}
	if !strings.Contains(out, "Condition not met (single check, no timeout set)") {
		t.Errorf("missing single-check banner:\n%s", out)
	}
	if strings.Contains(out, "attempt 2:") {
		t.Errorf("timeout=0 should perform exactly one check:\n%s", out)
	}
}

// T3 — interval > timeout warns and checks once.
func TestEdgeIntervalGreaterThanTimeout(t *testing.T) {
	ln, port := startListener(t)
	ln.Close()

	out, err := runRoot(t,
		"--timeout", "200ms",
		"--interval", "5s",
		"port", port,
	)
	if code := codeOf(t, err); code != exitcode.Timeout {
		t.Fatalf("exit = %d, want %d", code, exitcode.Timeout)
	}
	if !strings.Contains(out, "warning:") ||
		!strings.Contains(out, "longer than timeout") {
		t.Errorf("missing interval>timeout warning:\n%s", out)
	}
}

// T4 — interval below MinInterval is clamped.
func TestEdgeIntervalClampedToMin(t *testing.T) {
	ln, port := startListener(t)
	ln.Close()

	out, err := runRoot(t,
		"--verbose",
		"--timeout", "350ms",
		"--interval", "1ms",
		"port", port,
	)
	if code := codeOf(t, err); code != exitcode.Timeout {
		t.Fatalf("exit = %d, want %d", code, exitcode.Timeout)
	}
	// Clamped to 100ms → expect ~3-4 attempts in 350ms, not hundreds.
	count := strings.Count(out, "attempt ")
	if count > 6 {
		t.Errorf("attempts = %d; interval likely not clamped:\n%s", count, out)
	}
	if count < 2 {
		t.Errorf("attempts = %d; expected at least 2", count)
	}
}

// T5 — signal interrupts polling cleanly with exit 1.
//
// Uses SIGUSR1, which neither the Go runtime nor the test framework
// consumes by default. The mechanism (signal -> ctx cancel -> Interrupted)
// is the same as SIGINT/SIGTERM in production.
func TestEdgeSignalInterruptsPolling(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("signal test is Unix-only")
	}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGUSR1)
	defer cancel()

	root := NewRootCmd()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs([]string{
		"--timeout", "10s",
		"--interval", "200ms",
		"port", "9999",
	})

	done := make(chan error, 1)
	go func() {
		done <- root.ExecuteContext(ctx)
	}()

	time.Sleep(300 * time.Millisecond)

	if err := syscall.Kill(os.Getpid(), syscall.SIGUSR1); err != nil {
		t.Fatalf("kill: %v", err)
	}

	select {
	case err := <-done:
		ce, ok := errors.AsType[*exitcode.Error](err)
		if !ok {
			t.Fatalf("expected coded error, got %T: %v", err, err)
		}
		if ce.Code != exitcode.Timeout {
			t.Errorf("exit = %d, want %d", ce.Code, exitcode.Timeout)
		}
		if !strings.Contains(out.String(), "✗ Interrupted") {
			t.Errorf("missing interrupt banner:\n%s", out.String())
		}
	case <-time.After(5 * time.Second):
		t.Fatal("polling did not stop after signal")
	}
}

// T6 — --status 0 means "any 2xx".
func TestEdgeHTTPStatusZeroIsAny2xx(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	_, err := runRoot(t, "--timeout", "2s", "http", srv.URL, "--status", "0")
	if code := codeOf(t, err); code != 0 {
		t.Errorf("exit = %d, want 0", code)
	}
}

// T8 — all with mixed kinds, one succeeds, one times out.
func TestEdgeAllMixedKinds(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "ready")
	if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	ln, port := startListener(t)
	ln.Close()

	out, err := runRoot(t,
		"--timeout", "300ms",
		"--interval", "80ms",
		"all", "file", path, "port", port,
	)
	if code := codeOf(t, err); code != exitcode.Timeout {
		t.Fatalf("exit = %d, want %d\n%s", code, exitcode.Timeout, out)
	}
	if !strings.Contains(out, "1 of 2 conditions failed") {
		t.Errorf("missing failure count:\n%s", out)
	}
}

// T9 — a bad sub-condition inside all -> exit 2.
func TestEdgeAllBadSubConditionExit2(t *testing.T) {
	_, err := runRoot(t, "all", "port", "5432", "process", "1234")
	if code := codeOf(t, err); code != exitcode.BadArgs {
		t.Errorf("exit = %d, want %d", code, exitcode.BadArgs)
	}
}

// T9b — unit test for pickExitCode priority.
func TestPickExitCode(t *testing.T) {
	fatal := errors.New("boom")
	cases := []struct {
		name string
		in   []poller.Result
		want int
	}{
		{"all success", []poller.Result{{Success: true}, {Success: true}}, exitcode.Success},
		{"one timeout", []poller.Result{{Success: true}, {Success: false}}, exitcode.Timeout},
		{"two timeouts", []poller.Result{{Success: false}, {Success: false}}, exitcode.Timeout},
		{"fatal beats timeout", []poller.Result{
			{Success: false},
			{Success: false, Err: fatal},
		}, exitcode.BadArgs},
		{"fatal only", []poller.Result{{Success: false, Err: fatal}}, exitcode.BadArgs},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := pickExitCode(c.in); got != c.want {
				t.Errorf("got %d, want %d", got, c.want)
			}
		})
	}
}

// T10 — negative --timeout / --interval -> exit 2.
func TestEdgeNegativeTimeoutExit2(t *testing.T) {
	_, err := runRoot(t, "--timeout", "-5s", "port", "5432")
	if code := codeOf(t, err); code != exitcode.BadArgs {
		t.Errorf("exit = %d, want %d", code, exitcode.BadArgs)
	}
}

func TestEdgeNegativeIntervalExit2(t *testing.T) {
	_, err := runRoot(t, "--interval", "-5s", "port", "5432")
	if code := codeOf(t, err); code != exitcode.BadArgs {
		t.Errorf("exit = %d, want %d", code, exitcode.BadArgs)
	}
}

func TestEdgeAllNegativeTimeoutExit2(t *testing.T) {
	_, err := runRoot(t, "all", "port", "5432", "--timeout", "-5s")
	if code := codeOf(t, err); code != exitcode.BadArgs {
		t.Errorf("exit = %d, want %d", code, exitcode.BadArgs)
	}
}

// T11 — quiet preserves the timeout exit code.
func TestEdgeQuietTimeoutExitCode(t *testing.T) {
	ln, port := startListener(t)
	ln.Close()

	out, err := runRoot(t,
		"--quiet",
		"--timeout", "200ms",
		"--interval", "50ms",
		"port", port,
	)
	if code := codeOf(t, err); code != exitcode.Timeout {
		t.Errorf("exit = %d, want %d", code, exitcode.Timeout)
	}
	if out != "" {
		t.Errorf("expected quiet output, got %q", out)
	}
}

// T13 — global flag after a sub-condition inside all.
func TestEdgeAllGlobalFlagAfterCondition(t *testing.T) {
	ln, port := startListener(t)
	defer ln.Close()

	out, err := runRoot(t, "all", "port", port, "--timeout", "3s")
	if code := codeOf(t, err); code != 0 {
		t.Fatalf("exit = %d, err = %v\n%s", code, err, out)
	}
}

// T15 — sleep command interrupted mid-run still reports a state.
func TestEdgeCommandStillRunningState(t *testing.T) {
	out, err := runRoot(t,
		"--timeout", "300ms",
		"command", "sleep 100",
	)
	if code := codeOf(t, err); code != exitcode.Timeout {
		t.Fatalf("exit = %d, want %d\n%s", code, exitcode.Timeout, out)
	}
	if !strings.Contains(out, "still running") {
		t.Errorf("expected 'still running' state:\n%s", out)
	}
	if strings.Contains(out, "Last state: \n") || strings.Contains(out, "Last state:\n") {
		t.Errorf("Last state is blank:\n%s", out)
	}
}
