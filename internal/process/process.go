// Package process implements waitfor conditions that watch running processes.
//
// The scan is platform-specific (scan_linux.go, scan_darwin.go). The match
// logic lives here so it can be unit-tested without OS interaction.
package process

import (
	"context"
	"fmt"
	"os/user"
	"strconv"
	"strings"
)

// ProcInfo describes one running process.
type ProcInfo struct {
	PID  int
	Name string // short process name
	User string // owner username; empty if unknown
}

// ProcessCondition waits for a process matching a name to be running.
type ProcessCondition struct {
	name  string
	exact bool
	user  string
}

// ProcessOptions configures NewProcessCondition.
type ProcessOptions struct {
	Exact bool
	User  string
}

// NewProcessCondition validates the name and options.
//
// A purely numeric name is rejected with a hint to use
// `waitfor command 'kill -0 <pid>'` instead.
func NewProcessCondition(name string, opts ProcessOptions) (*ProcessCondition, error) {
	if name == "" {
		return nil, fmt.Errorf("process: empty name")
	}
	if _, err := strconv.Atoi(name); err == nil {
		return nil, fmt.Errorf(
			"process: %q looks like a PID, not a name.\n"+
				"       To wait for a specific PID, use: waitfor command 'kill -0 %s'",
			name, name,
		)
	}

	normalizedUser := ""
	if opts.User != "" {
		u, err := normalizeUser(opts.User)
		if err != nil {
			return nil, err
		}
		normalizedUser = u
	}

	return &ProcessCondition{
		name:  name,
		exact: opts.Exact,
		user:  normalizedUser,
	}, nil
}

// normalizeUser resolves a username or UID string to a username.
func normalizeUser(input string) (string, error) {
	if u, err := user.Lookup(input); err == nil {
		return u.Username, nil
	}
	if u, err := user.LookupId(input); err == nil {
		return u.Username, nil
	}
	return "", fmt.Errorf("process: unknown user %q", input)
}

func (c *ProcessCondition) Kind() string   { return "process" }
func (c *ProcessCondition) Target() string { return c.name }
func (c *ProcessCondition) Describe() string {
	if c.user != "" {
		return fmt.Sprintf("process %q (user %s)", c.name, c.user)
	}
	return fmt.Sprintf("process %q", c.name)
}
func (c *ProcessCondition) SuccessMessage() string {
	return fmt.Sprintf("Process %q is running", c.name)
}

// Check scans running processes and reports the first match.
//
// Failure states produced:
//   - "no matching process found"
//   - listProcesses error (fatal, e.g. unsupported platform)
func (c *ProcessCondition) Check(_ context.Context) (string, bool, error) {
	procs, err := listProcesses()
	if err != nil {
		return err.Error(), false, err
	}
	for _, p := range procs {
		if procMatches(p, c.name, c.exact, c.user) {
			return fmt.Sprintf("matched pid %d (%s)", p.PID, p.Name), true, nil
		}
	}
	return "no matching process found", false, nil
}

// procMatches is the pure match function, extracted for unit testing.
//
// When user is non-empty, processes with an unknown owner are treated as
// non-matches (conservative: we can't confirm they belong to the user).
func procMatches(p ProcInfo, name string, exact bool, username string) bool {
	if username != "" {
		if p.User == "" || p.User != username {
			return false
		}
	}
	if exact {
		return p.Name == name
	}
	return strings.Contains(p.Name, name)
}
