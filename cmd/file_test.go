package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/0xProgress/waitfor/internal/exitcode"
)

func TestFileCmdSuccess(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "ready")
	if err := os.WriteFile(path, []byte("hi"), 0o644); err != nil {
		t.Fatal(err)
	}

	out, err := runRoot(t, "--timeout", "2s", "file", path)
	if code := codeOf(t, err); code != 0 {
		t.Fatalf("exit = %d, err = %v\n%s", code, err, out)
	}
	if !strings.Contains(out, "✓ "+path+" exists") {
		t.Errorf("missing success line:\n%s", out)
	}
}

func TestFileCmdTimeout(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "never")

	out, err := runRoot(t,
		"--timeout", "250ms",
		"--interval", "60ms",
		"file", path,
	)
	if code := codeOf(t, err); code != exitcode.Timeout {
		t.Fatalf("exit = %d, want %d\n%s", code, exitcode.Timeout, out)
	}
	for _, want := range []string{
		"✗ Timed out after",
		"file exists " + path,
		"not found",
		"Tip:",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
}

func TestFileCmdParentMissing(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nope", "ready")

	out, err := runRoot(t,
		"--timeout", "200ms",
		"--interval", "60ms",
		"file", path,
	)
	if code := codeOf(t, err); code != exitcode.Timeout {
		t.Fatalf("exit = %d, want %d", code, exitcode.Timeout)
	}
	if !strings.Contains(out, "parent directory missing") {
		t.Errorf("missing parent-missing state:\n%s", out)
	}
}

func TestFileCmdNonEmpty(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "empty")
	if err := os.WriteFile(path, nil, 0o644); err != nil {
		t.Fatal(err)
	}

	out, err := runRoot(t,
		"--timeout", "200ms",
		"--interval", "60ms",
		"file", path, "--nonempty",
	)
	if code := codeOf(t, err); code != exitcode.Timeout {
		t.Fatalf("exit = %d, want %d", code, exitcode.Timeout)
	}
	if !strings.Contains(out, "empty") {
		t.Errorf("missing empty state:\n%s", out)
	}
}

func TestFileCmdMinSize(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "data")
	if err := os.WriteFile(path, []byte("abc"), 0o644); err != nil {
		t.Fatal(err)
	}

	out, err := runRoot(t,
		"--timeout", "200ms",
		"--interval", "60ms",
		"file", path, "--min-size", "100",
	)
	if code := codeOf(t, err); code != exitcode.Timeout {
		t.Fatalf("exit = %d, want %d\n%s", code, exitcode.Timeout, out)
	}
	if !strings.Contains(out, "size 3") {
		t.Errorf("missing size state:\n%s", out)
	}
}

func TestFileCmdDirectory(t *testing.T) {
	dir := t.TempDir()

	out, err := runRoot(t, "--timeout", "2s", "file", dir)
	if code := codeOf(t, err); code != 0 {
		t.Fatalf("exit = %d, err = %v\n%s", code, err, out)
	}
	if !strings.Contains(out, "✓ "+dir+" exists") {
		t.Errorf("missing success line:\n%s", out)
	}
}

func TestFileCmdJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "ready")
	if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	out, err := runRoot(t, "--json", "--timeout", "2s", "file", path)
	if code := codeOf(t, err); code != 0 {
		t.Fatalf("exit = %d, err = %v", code, err)
	}
	for _, want := range []string{
		`"condition": "file"`,
		`"target": "` + path + `"`,
		`"success": true`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
}

func TestFileCmdPollsUntilAppears(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sentinel")

	go func() {
		time.Sleep(120 * time.Millisecond)
		_ = os.WriteFile(path, []byte("ready"), 0o644)
	}()

	out, err := runRoot(t,
		"--timeout", "2s",
		"--interval", "50ms",
		"file", path,
	)
	if code := codeOf(t, err); code != 0 {
		t.Fatalf("exit = %d, err = %v\n%s", code, err, out)
	}
	if !strings.Contains(out, "✓") {
		t.Errorf("missing success line:\n%s", out)
	}
}
