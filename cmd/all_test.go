package cmd

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/0xProgress/waitfor/internal/exitcode"
)

func TestAllCmdAllSucceed(t *testing.T) {
	ln1, port1 := startListener(t)
	defer ln1.Close()
	ln2, port2 := startListener(t)
	defer ln2.Close()

	out, err := runRoot(t, "--timeout", "3s",
		"all", "port", port1, "port", port2)
	if code := codeOf(t, err); code != 0 {
		t.Fatalf("exit = %d, err = %v\n%s", code, err, out)
	}
	if !strings.Contains(out, "✓ all conditions met") {
		t.Errorf("missing success line:\n%s", out)
	}
}

func TestAllCmdOneFails(t *testing.T) {
	ln, port := startListener(t)
	defer ln.Close()

	ln2, port2 := startListener(t)
	ln2.Close() // nothing listening now

	out, err := runRoot(t,
		"--timeout", "400ms",
		"--interval", "80ms",
		"all", "port", port, "port", port2)
	if code := codeOf(t, err); code != exitcode.Timeout {
		t.Fatalf("exit = %d, want %d\n%s", code, exitcode.Timeout, out)
	}
	if !strings.Contains(out, "1 of 2 conditions failed") {
		t.Errorf("missing failure count:\n%s", out)
	}
	if !strings.Contains(out, "TCP localhost:"+port2) {
		t.Errorf("missing failing condition describe:\n%s", out)
	}
}

func TestAllCmdZeroConditionsExit2(t *testing.T) {
	_, err := runRoot(t, "all")
	if code := codeOf(t, err); code != exitcode.BadArgs {
		t.Errorf("exit = %d, want %d", code, exitcode.BadArgs)
	}
}

func TestAllCmdUnknownKeywordExit2(t *testing.T) {
	_, err := runRoot(t, "all", "bogus", "arg")
	if code := codeOf(t, err); code != exitcode.BadArgs {
		t.Errorf("exit = %d, want %d", code, exitcode.BadArgs)
	}
}

func TestAllCmdMalformedSubConditionExit2(t *testing.T) {
	// `port` without an argument, immediately followed by another keyword.
	_, err := runRoot(t, "all", "port", "port", "5432")
	if code := codeOf(t, err); code != exitcode.BadArgs {
		t.Errorf("exit = %d, want %d", code, exitcode.BadArgs)
	}
}

func TestAllCmdPerConditionFlags(t *testing.T) {
	srv1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv1.Close()
	srv2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv2.Close()

	// Different --status for each http condition proves flags are scoped.
	out, err := runRoot(t,
		"--timeout", "3s",
		"all",
		"http", srv1.URL, "--status", "200",
		"http", srv2.URL, "--status", "500",
	)
	if code := codeOf(t, err); code != 0 {
		t.Fatalf("exit = %d, err = %v\n%s", code, err, out)
	}
	if !strings.Contains(out, "✓ all conditions met") {
		t.Errorf("missing success line:\n%s", out)
	}
}

func TestAllCmdFileNonEmptyPerCondition(t *testing.T) {
	dir := t.TempDir()
	empty := filepath.Join(dir, "empty")
	full := filepath.Join(dir, "full")
	if err := os.WriteFile(empty, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	// empty with no flag → passes existence; full with --nonempty → passes.
	out, err := runRoot(t, "--timeout", "2s",
		"all",
		"file", empty,
		"file", full, "--nonempty",
	)
	if code := codeOf(t, err); code != 0 {
		t.Fatalf("exit = %d, err = %v\n%s", code, err, out)
	}
}

func TestAllCmdRunsInParallel(t *testing.T) {
	dir := t.TempDir()
	f1 := filepath.Join(dir, "f1")
	f2 := filepath.Join(dir, "f2")

	go func() {
		time.Sleep(300 * time.Millisecond)
		_ = os.WriteFile(f1, []byte("a"), 0o644)
	}()
	go func() {
		time.Sleep(400 * time.Millisecond)
		_ = os.WriteFile(f2, []byte("b"), 0o644)
	}()

	start := time.Now()
	_, err := runRoot(t,
		"--timeout", "3s",
		"--interval", "50ms",
		"all", "file", f1, "file", f2)
	elapsed := time.Since(start)
	if code := codeOf(t, err); code != 0 {
		t.Fatalf("exit = %d", code)
	}
	// Parallel: ~400ms. Serial: ~700ms. Allow generous headroom.
	if elapsed > 650*time.Millisecond {
		t.Errorf("elapsed %s; expected parallel (~400ms), not serial", elapsed)
	}
}

func TestAllCmdJSON(t *testing.T) {
	ln1, port1 := startListener(t)
	defer ln1.Close()

	ln2, port2 := startListener(t)
	ln2.Close()

	out, err := runRoot(t,
		"--json",
		"--timeout", "300ms",
		"--interval", "80ms",
		"all", "port", port1, "port", port2)
	if code := codeOf(t, err); code != exitcode.Timeout {
		t.Fatalf("exit = %d", code)
	}

	var got map[string]any
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out)
	}
	if got["condition"] != "all" {
		t.Errorf("condition = %v", got["condition"])
	}
	if got["success"] != false {
		t.Errorf("success = %v", got["success"])
	}
	children, ok := got["children"].([]any)
	if !ok {
		t.Fatalf("children not array: %T", got["children"])
	}
	if len(children) != 2 {
		t.Fatalf("children len = %d, want 2", len(children))
	}
	c0 := children[0].(map[string]any)
	c1 := children[1].(map[string]any)
	if c0["target"] != port1 || c0["success"] != true {
		t.Errorf("child 0 = %v", c0)
	}
	if c1["target"] != port2 || c1["success"] != false {
		t.Errorf("child 1 = %v", c1)
	}
}

func TestAllCmdVerboseShowsProgress(t *testing.T) {
	ln, port := startListener(t)
	ln.Close()

	out, err := runRoot(t,
		"--verbose",
		"--timeout", "300ms",
		"--interval", "80ms",
		"all", "port", port)
	if code := codeOf(t, err); code != exitcode.Timeout {
		t.Fatalf("exit = %d", code)
	}
	if !strings.Contains(out, "attempt") {
		t.Errorf("verbose output missing attempt lines:\n%s", out)
	}
}

func TestAllCmdHelp(t *testing.T) {
	out, err := runRoot(t, "all", "--help")
	if err != nil {
		t.Fatalf("--help returned error: %v", err)
	}
	for _, want := range []string{"all <condition>", "Global flags"} {
		if !strings.Contains(out, want) {
			t.Errorf("help missing %q:\n%s", want, out)
		}
	}
}
