package command

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestNewCommandConditionRejectsEmpty(t *testing.T) {
	for _, s := range []string{"", "   ", "\t\n"} {
		if _, err := NewCommandCondition(s, CommandOptions{}); err == nil {
			t.Errorf("NewCommandCondition(%q) = nil error", s)
		}
	}
}

func TestCommandConditionMeta(t *testing.T) {
	c, _ := NewCommandCondition("echo hi", CommandOptions{})
	if c.Kind() != "command" {
		t.Errorf("Kind = %q", c.Kind())
	}
	if c.Target() != "echo hi" {
		t.Errorf("Target = %q", c.Target())
	}
	if c.Describe() != `command "echo hi"` {
		t.Errorf("Describe = %q", c.Describe())
	}
	if !strings.Contains(c.SuccessMessage(), "exit") {
		t.Errorf("SuccessMessage = %q", c.SuccessMessage())
	}
}

func TestCommandCheckTrue(t *testing.T) {
	c, _ := NewCommandCondition("true", CommandOptions{})
	state, ok, err := c.Check(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatalf("expected success, state = %q", state)
	}
	if state != "exit code 0" {
		t.Errorf("state = %q", state)
	}
}

func TestCommandCheckFalse(t *testing.T) {
	c, _ := NewCommandCondition("false", CommandOptions{})
	state, ok, err := c.Check(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Fatal("expected failure")
	}
	if state != "exit code 1" {
		t.Errorf("state = %q, want exit code 1", state)
	}
}

func TestCommandCustomExitCode(t *testing.T) {
	c, _ := NewCommandCondition("exit 7", CommandOptions{ExitCode: 7})
	if _, ok, _ := c.Check(context.Background()); !ok {
		t.Fatal("exit 7 should match wantCode 7")
	}

	c2, _ := NewCommandCondition("exit 7", CommandOptions{ExitCode: 3})
	if _, ok, _ := c2.Check(context.Background()); ok {
		t.Fatal("exit 7 should not match wantCode 3")
	}
}

func TestCommandCapturesStderr(t *testing.T) {
	c, _ := NewCommandCondition(`echo "boom happened" >&2; exit 1`, CommandOptions{})
	if _, ok, _ := c.Check(context.Background()); ok {
		t.Fatal("expected failure")
	}
	label, val := c.LastDetail()
	if label != "Stderr" {
		t.Errorf("label = %q, want Stderr", label)
	}
	if val != "boom happened" {
		t.Errorf("stderr = %q", val)
	}
}

func TestCommandClearsStderrOnSuccess(t *testing.T) {
	// Run a command that emits stderr but succeeds.
	c, _ := NewCommandCondition(`echo "noise" >&2; true`, CommandOptions{})
	if _, ok, _ := c.Check(context.Background()); !ok {
		t.Fatal("expected success")
	}
	if _, val := c.LastDetail(); val != "" {
		t.Errorf("stderr after success = %q, want empty", val)
	}
}

func TestCommandPipesWork(t *testing.T) {
	c, _ := NewCommandCondition(`echo hello | grep hello`, CommandOptions{})
	if _, ok, _ := c.Check(context.Background()); !ok {
		t.Fatal("pipe should succeed")
	}
}

func TestCommandRedirectionWorks(t *testing.T) {
	c, _ := NewCommandCondition(`echo x > /dev/null`, CommandOptions{})
	if _, ok, _ := c.Check(context.Background()); !ok {
		t.Fatal("redirection should succeed")
	}
}

func TestCommandContextCancel(t *testing.T) {
	// A command that sleeps longer than the ctx allows.
	c, _ := NewCommandCondition(`sleep 5`, CommandOptions{})
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	start := time.Now()
	state, ok, err := c.Check(ctx)
	elapsed := time.Since(start)

	if ok {
		t.Fatalf("expected failure, state = %q", state)
	}
	if err != nil {
		// A non-nil error is acceptable; the poller treats it as a timeout
		// when ctx is dead.
		t.Logf("Check returned err: %v", err)
	}
	if elapsed > 2*time.Second {
		t.Errorf("took %s; ctx should have killed the command quickly", elapsed)
	}
}
