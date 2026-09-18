package main

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"github.com/0xProgress/waitfor/internal/exitcode"
)

func TestRunContextHelpReturnsZero(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := runContext(context.Background(), []string{"--help"}, &stdout, &stderr)
	if code != exitcode.Success {
		t.Errorf("exit = %d, want %d\nstderr: %s", code, exitcode.Success, stderr.String())
	}
	if !strings.Contains(stdout.String(), "Block until a condition is true") {
		t.Errorf("help output missing tagline:\n%s", stdout.String())
	}
}

func TestRunContextNoSubcommandExit2(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := runContext(context.Background(), nil, &stdout, &stderr)
	if code != exitcode.BadArgs {
		t.Errorf("exit = %d, want %d", code, exitcode.BadArgs)
	}
}

func TestRunContextUnknownSubcommandExit2(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := runContext(context.Background(), []string{"nonsense"}, &stdout, &stderr)
	if code != exitcode.BadArgs {
		t.Errorf("exit = %d, want %d", code, exitcode.BadArgs)
	}
	if !strings.Contains(stderr.String(), "waitfor: ") {
		t.Errorf("expected 'waitfor: ' prefix on stderr:\n%s", stderr.String())
	}
}

func TestRunContextTimeoutExit1(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := runContext(context.Background(),
		[]string{"--timeout", "100ms", "--interval", "50ms", "port", "1"},
		&stdout, &stderr)
	if code != exitcode.Timeout {
		t.Errorf("exit = %d, want %d", code, exitcode.Timeout)
	}
	// The failure block is rendered to stdout by the output layer.
	if !strings.Contains(stdout.String(), "✗") {
		t.Errorf("expected failure banner on stdout:\n%s", stdout.String())
	}
	// SilentTimeout: main must NOT prepend an extra "waitfor: " line.
	if strings.Contains(stderr.String(), "waitfor: ") {
		t.Errorf("silent timeout should not write extra stderr:\n%s", stderr.String())
	}
}

func TestRunContextInvalidPortExit2(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := runContext(context.Background(), []string{"port", "0"}, &stdout, &stderr)
	if code != exitcode.BadArgs {
		t.Errorf("exit = %d, want %d", code, exitcode.BadArgs)
	}
	if !strings.Contains(stderr.String(), "waitfor: ") {
		t.Errorf("expected 'waitfor: ' prefix on stderr:\n%s", stderr.String())
	}
	if !strings.Contains(stderr.String(), "invalid port") {
		t.Errorf("expected invalid-port message on stderr:\n%s", stderr.String())
	}
}

func TestRunContextQuietTimeoutExit1(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := runContext(context.Background(),
		[]string{"--quiet", "--timeout", "100ms", "--interval", "50ms", "port", "1"},
		&stdout, &stderr)
	if code != exitcode.Timeout {
		t.Errorf("exit = %d, want %d", code, exitcode.Timeout)
	}
	if stdout.Len() != 0 {
		t.Errorf("--quiet should suppress stdout, got %q", stdout.String())
	}
	if stderr.Len() != 0 {
		t.Errorf("--quiet silent timeout should suppress stderr, got %q", stderr.String())
	}
}

func TestRunContextContextCancelReturnsInterrupted(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		<-time.After(50 * time.Millisecond)
		cancel()
	}()

	var stdout, stderr bytes.Buffer
	code := runContext(ctx,
		[]string{"--timeout", "10s", "--interval", "200ms", "port", "1"},
		&stdout, &stderr)
	if code != exitcode.Timeout {
		t.Errorf("exit = %d, want %d", code, exitcode.Timeout)
	}
	if !strings.Contains(stdout.String(), "✗ Interrupted") {
		t.Errorf("expected interrupted banner:\n%s", stdout.String())
	}
}

// timeAfter is a tiny helper so the test file doesn't import time just for
// one sleep; keeping imports minimal here.
func timeAfter(ms int) <-chan struct{} {
	ch := make(chan struct{})
	go func() {
		defer close(ch)
		// A simple busy-wait is overkill; use a duration-based sleep via
		// the standard library in the goroutine. But we want zero imports.
		// So: yield via runtime.Gosched loops + counting is fragile.
		// Simplest: use time.Sleep — accept the import.
		_ = ms
		_ = ch
	}()
	return ch
}
