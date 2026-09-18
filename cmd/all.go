package cmd

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/spf13/cobra"

	"github.com/0xProgress/waitfor/internal/command"
	"github.com/0xProgress/waitfor/internal/exitcode"
	"github.com/0xProgress/waitfor/internal/file"
	"github.com/0xProgress/waitfor/internal/network"
	"github.com/0xProgress/waitfor/internal/output"
	"github.com/0xProgress/waitfor/internal/poller"
	"github.com/0xProgress/waitfor/internal/process"
)

func newAllCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "all <condition> [condition...]",
		Short: "Wait for several conditions at once; all must pass",
		Long: `Wait for multiple conditions simultaneously. All must pass.

Global flags (--timeout, --interval, --quiet, --json, --verbose) may appear
anywhere in the argument list and apply to all sub-conditions.

Each sub-condition is a keyword (port, http, file, process, command)
followed by that subcommand's positional arguments and flags.

Examples:
  waitfor all port 5432 port 6379
  waitfor all port 5432 --timeout 3s
  waitfor all http localhost:8080/health --status 200 file ./ready
  waitfor all --timeout 60s port 5432 process postgres`,
		DisableFlagParsing: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAll(cmd, args)
		},
	}
}

// runAll is the entry point for `waitfor all`.
func runAll(cmd *cobra.Command, args []string) error {
	if len(args) > 0 && (args[0] == "--help" || args[0] == "-h") {
		return cmd.Help()
	}

	subArgs, err := consumeGlobalFlags(args)
	if err != nil {
		return exitcode.BadArgsf("%s", err)
	}
	if len(subArgs) == 0 {
		return exitcode.BadArgsf("all: no conditions specified")
	}

	conditions, err := splitConditions(subArgs)
	if err != nil {
		return exitcode.BadArgsf("all: %s", err)
	}

	return runAllConditions(cmd, conditions)
}

// consumeGlobalFlags scans args for global flags anywhere and applies them,
// returning the args with those flags removed.
func consumeGlobalFlags(args []string) ([]string, error) {
	var out []string
	for i := 0; i < len(args); i++ {
		a := args[i]

		if a == "--" {
			out = append(out, args[i+1:]...)
			break
		}

		name, inlineValue, hasInline := splitFlag(a)
		switch name {
		case "--timeout":
			v, ni, err := flagValue(args, i, inlineValue, hasInline, name)
			if err != nil {
				return nil, err
			}
			d, err := time.ParseDuration(v)
			if err != nil {
				return nil, fmt.Errorf("invalid --timeout %q: %w", v, err)
			}
			if d < 0 {
				return nil, fmt.Errorf("--timeout must be >= 0, got %s", d)
			}
			flagTimeout = d
			i = ni
		case "--interval":
			v, ni, err := flagValue(args, i, inlineValue, hasInline, name)
			if err != nil {
				return nil, err
			}
			d, err := time.ParseDuration(v)
			if err != nil {
				return nil, fmt.Errorf("invalid --interval %q: %w", v, err)
			}
			if d < 0 {
				return nil, fmt.Errorf("--interval must be >= 0, got %s", d)
			}
			flagInterval = d
			i = ni
		case "--quiet":
			flagQuiet = true
		case "--json":
			flagJSON = true
		case "--verbose":
			flagVerbose = true
		default:
			out = append(out, a)
		}
	}
	return out, nil
}

// splitFlag returns (name, value, hasInlineValue) for a --flag or --flag=value.
// For non-flags, name is "".
func splitFlag(a string) (name, value string, hasInline bool) {
	if !strings.HasPrefix(a, "--") {
		return "", "", false
	}
	name, value, hasInline = strings.Cut(a, "=")
	return name, value, hasInline
}

// flagValue resolves the value of a flag at args[i].
func flagValue(args []string, i int, inlineValue string, hasInline bool, name string) (string, int, error) {
	if hasInline {
		return inlineValue, i, nil
	}
	if i+1 >= len(args) {
		return "", 0, fmt.Errorf("flag %s requires a value", name)
	}
	return args[i+1], i + 1, nil
}

// isConditionKeyword reports whether s names a subcommand that can appear
// inside `all`.
func isConditionKeyword(s string) bool {
	switch s {
	case "port", "http", "file", "process", "command":
		return true
	}
	return false
}

// splitConditions walks args and groups tokens into sub-conditions.
func splitConditions(args []string) ([]poller.Condition, error) {
	var conditions []poller.Condition
	var current []string

	for _, a := range args {
		if current == nil {
			if !isConditionKeyword(a) {
				return nil, fmt.Errorf(
					"expected a condition keyword (port, http, file, process, command), got %q",
					a,
				)
			}
			current = []string{a}
			continue
		}

		if isConditionKeyword(a) {
			if cond, err := buildSubCondition(current); err == nil {
				conditions = append(conditions, cond)
				current = []string{a}
				continue
			}
		}
		current = append(current, a)
	}

	if current == nil {
		return nil, errors.New("no conditions specified")
	}
	cond, err := buildSubCondition(current)
	if err != nil {
		return nil, err
	}
	conditions = append(conditions, cond)
	return conditions, nil
}

// buildSubCondition parses one sub-condition slice and returns the condition.
func buildSubCondition(slice []string) (poller.Condition, error) {
	if len(slice) == 0 {
		return nil, errors.New("empty condition")
	}
	kind := slice[0]
	rest := slice[1:]

	switch kind {
	case "port":
		return buildPortCondition(rest)
	case "http":
		return buildHTTPCondition(rest)
	case "file":
		return buildFileCondition(rest)
	case "process":
		return buildProcessCondition(rest)
	case "command":
		return buildCommandCondition(rest)
	}
	return nil, fmt.Errorf("unknown condition %q", kind)
}

func buildPortCondition(rest []string) (poller.Condition, error) {
	c := newPortCmd()
	if err := c.Flags().Parse(rest); err != nil {
		return nil, fmt.Errorf("port: %w", err)
	}
	pos := c.Flags().Args()
	if len(pos) != 1 {
		return nil, fmt.Errorf("port: expected exactly 1 argument, got %d", len(pos))
	}
	return network.NewPortCondition(pos[0])
}

func buildHTTPCondition(rest []string) (poller.Condition, error) {
	c := newHTTPCmd()
	if err := c.Flags().Parse(rest); err != nil {
		return nil, fmt.Errorf("http: %w", err)
	}
	pos := c.Flags().Args()
	if len(pos) != 1 {
		return nil, fmt.Errorf("http: expected exactly 1 argument, got %d", len(pos))
	}
	status, _ := c.Flags().GetInt("status")
	method, _ := c.Flags().GetString("method")
	insecure, _ := c.Flags().GetBool("insecure")
	headers, _ := c.Flags().GetStringArray("header")
	return network.NewHTTPCondition(pos[0], network.HTTPOptions{
		Method:   method,
		WantCode: status,
		Insecure: insecure,
		Headers:  headers,
	})
}

func buildFileCondition(rest []string) (poller.Condition, error) {
	c := newFileCmd()
	if err := c.Flags().Parse(rest); err != nil {
		return nil, fmt.Errorf("file: %w", err)
	}
	pos := c.Flags().Args()
	if len(pos) != 1 {
		return nil, fmt.Errorf("file: expected exactly 1 argument, got %d", len(pos))
	}
	minSize, _ := c.Flags().GetInt64("min-size")
	nonEmpty, _ := c.Flags().GetBool("nonempty")
	return file.NewFileCondition(pos[0], file.FileOptions{
		MinSize:  minSize,
		NonEmpty: nonEmpty,
	})
}

func buildProcessCondition(rest []string) (poller.Condition, error) {
	c := newProcessCmd()
	if err := c.Flags().Parse(rest); err != nil {
		return nil, fmt.Errorf("process: %w", err)
	}
	pos := c.Flags().Args()
	if len(pos) != 1 {
		return nil, fmt.Errorf("process: expected exactly 1 argument, got %d", len(pos))
	}
	exact, _ := c.Flags().GetBool("exact")
	userFlag, _ := c.Flags().GetString("user")
	return process.NewProcessCondition(pos[0], process.ProcessOptions{
		Exact: exact,
		User:  userFlag,
	})
}

func buildCommandCondition(rest []string) (poller.Condition, error) {
	c := newCommandCmd()
	if err := c.Flags().Parse(rest); err != nil {
		return nil, fmt.Errorf("command: %w", err)
	}
	pos := c.Flags().Args()
	if len(pos) != 1 {
		return nil, fmt.Errorf("command: expected exactly 1 argument, got %d", len(pos))
	}
	exitCode, _ := c.Flags().GetInt("exit-code")
	return command.NewCommandCondition(pos[0], command.CommandOptions{
		ExitCode: exitCode,
	})
}

// runAllConditions polls all conditions in parallel and renders the outcome.
func runAllConditions(cmd *cobra.Command, conditions []poller.Condition) error {
	out := cmd.OutOrStdout()
	start := time.Now()

	results := make([]poller.Result, len(conditions))
	var wg sync.WaitGroup
	var mu sync.Mutex

	for i, cond := range conditions {
		wg.Add(1)
		go func(i int, cond poller.Condition) {
			defer wg.Done()
			p := &poller.Poller{
				Timeout:  flagTimeout,
				Interval: flagInterval,
				Verbose:  flagVerbose,
			}
			if flagVerbose && !flagQuiet && !flagJSON {
				label := cond.Describe()
				p.OnAttempt = func(n int, state string, elapsed time.Duration, ok bool) {
					mu.Lock()
					defer mu.Unlock()
					mark := "✗"
					if ok {
						mark = "✓"
					}
					fmt.Fprintf(out, "  %s [%s] attempt %d: %s (%s)\n",
						mark, label, n, state, output.FormatDuration(elapsed))
				}
			}
			results[i] = p.Run(cmd.Context(), cond)
		}(i, cond)
	}
	wg.Wait()

	elapsed := time.Since(start)
	allSuccess := true
	interrupted := false
	for _, r := range results {
		if !r.Success {
			allSuccess = false
		}
		if r.Interrupted {
			interrupted = true
		}
	}

	ar := output.AllResult{
		Children:    results,
		Success:     allSuccess,
		Elapsed:     elapsed,
		Timeout:     flagTimeout,
		Interrupted: interrupted,
	}

	opts := output.Options{
		Quiet:   flagQuiet,
		JSON:    flagJSON,
		Verbose: flagVerbose,
	}
	if err := output.RenderAll(out, ar, opts); err != nil {
		return &exitcode.Error{Code: exitcode.BadArgs, Err: err}
	}

	if allSuccess {
		return nil
	}
	// Details already rendered; return the highest-priority code.
	return &exitcode.Error{Code: pickExitCode(results), Silent: true}
}

// pickExitCode inspects the child results and returns the process exit code.
//
// Priority: BadArgs (2) > Timeout (1) > Success (0). A child with a non-nil
// Err is a fatal condition failure (bad input, environment problem), which
// outranks a plain timeout. Permission (3) is unreachable today because no
// condition surfaces permission errors from Check, but the helper is written
// to accommodate it if that changes.
func pickExitCode(results []poller.Result) int {
	code := exitcode.Success
	for _, r := range results {
		if r.Err != nil {
			return exitcode.BadArgs
		}
		if !r.Success && code == exitcode.Success {
			code = exitcode.Timeout
		}
	}
	return code
}
