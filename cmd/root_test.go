package cmd

import (
	"bytes"
	"errors"
	"testing"
	"time"

	"github.com/0xProgress/waitfor/internal/exitcode"
)

// runRoot executes the CLI with the given args and captures output.
// It returns the stdout buffer and the error (if any).
func runRoot(t *testing.T, args ...string) (string, error) {
	t.Helper()
	root := NewRootCmd()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs(args)
	err := root.Execute()
	return out.String(), Normalize(err)
}

func TestRootHasAllSubcommands(t *testing.T) {
	root := NewRootCmd()
	want := map[string]bool{
		"port":    false,
		"http":    false,
		"file":    false,
		"process": false,
		"command": false,
		"all":     false,
	}
	for _, c := range root.Commands() {
		if _, ok := want[c.Name()]; ok {
			want[c.Name()] = true
		}
	}
	for name, seen := range want {
		if !seen {
			t.Errorf("missing subcommand %q", name)
		}
	}
}

func TestGlobalFlagDefaults(t *testing.T) {
	root := NewRootCmd()
	pf := root.PersistentFlags()
	cases := []struct{ name, def string }{
		{"timeout", "30s"},
		{"interval", "500ms"},
		{"quiet", "false"},
		{"json", "false"},
		{"verbose", "false"},
	}
	for _, c := range cases {
		f := pf.Lookup(c.name)
		if f == nil {
			t.Errorf("missing global flag --%s", c.name)
			continue
		}
		if f.DefValue != c.def {
			t.Errorf("--%s default = %q, want %q", c.name, f.DefValue, c.def)
		}
	}
}

func TestGlobalFlagParsing(t *testing.T) {
	_, err := runRoot(t,
		"--timeout", "5s",
		"--interval", "2s",
		"--quiet",
		"--json",
		"port", "1234",
	)
	// Port subcommand is a stub in Phase 0 — that's fine, flags are parsed
	// before RunE fires.
	if err == nil {
		t.Fatal("expected stub error from port")
	}
	if flagTimeout != 5*time.Second {
		t.Errorf("flagTimeout = %v, want 5s", flagTimeout)
	}
	if flagInterval != 2*time.Second {
		t.Errorf("flagInterval = %v, want 2s", flagInterval)
	}
	if !flagQuiet {
		t.Error("flagQuiet not set")
	}
	if !flagJSON {
		t.Error("flagJSON not set")
	}
}

func TestNoSubcommandIsBadArgs(t *testing.T) {
	_, err := runRoot(t)
	if err == nil {
		t.Fatal("expected error for missing subcommand")
	}
	ce, ok := errors.AsType[*exitcode.Error](err)
	if !ok {
		t.Fatalf("expected *exitcode.Error, got %T: %v", err, err)
	}
	if ce.Code != exitcode.BadArgs {
		t.Errorf("code = %d, want %d", ce.Code, exitcode.BadArgs)
	}
}

func TestUnknownSubcommandIsBadArgs(t *testing.T) {
	_, err := runRoot(t, "nonsense")
	ce, ok := errors.AsType[*exitcode.Error](err)
	if !ok {
		t.Fatalf("expected *exitcode.Error, got %T: %v", err, err)
	}
	if ce.Code != exitcode.BadArgs {
		t.Errorf("code = %d, want %d", ce.Code, exitcode.BadArgs)
	}
}

func TestSubcommandFlagRegistration(t *testing.T) {
	root := NewRootCmd()
	want := map[string][]string{
		"http":    {"status", "method", "insecure", "header"},
		"file":    {"min-size", "nonempty"},
		"process": {"exact", "user"},
		"command": {"exit-code"},
	}
	for _, c := range root.Commands() {
		names, ok := want[c.Name()]
		if !ok {
			continue
		}
		for _, f := range names {
			if c.Flags().Lookup(f) == nil {
				t.Errorf("subcommand %q missing flag --%s", c.Name(), f)
			}
		}
	}
}
