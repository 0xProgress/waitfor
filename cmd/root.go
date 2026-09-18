// Package cmd wires up the waitfor command-line interface.
package cmd

import (
	"context"
	"errors"
	"time"

	"github.com/spf13/cobra"

	"github.com/0xProgress/waitfor/internal/exitcode"
	"github.com/0xProgress/waitfor/internal/version"
)

// Global flags, shared by all subcommands.
var (
	flagTimeout  time.Duration
	flagInterval time.Duration
	flagQuiet    bool
	flagJSON     bool
	flagVerbose  bool
)

// NewRootCmd builds the root command with all subcommands attached.
func NewRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "waitfor",
		Short: "Block until a condition is true, or tell you exactly why it wasn't.",
		Long: `Block until a condition is true, or tell you exactly why it wasn't.

waitfor polls a condition on a configurable interval until it becomes
true or a timeout expires. On failure it reports the last observed state
and a hint about what to check next.`,
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			// `all` uses DisableFlagParsing and consumes globals itself,
			// validating them inside consumeGlobalFlags.
			if cmd.Name() == "all" {
				return nil
			}
			if flagTimeout < 0 {
				return exitcode.BadArgsf("--timeout must be >= 0, got %s", flagTimeout)
			}
			if flagInterval < 0 {
				return exitcode.BadArgsf("--interval must be >= 0, got %s", flagInterval)
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				_ = cmd.Help()
				return exitcode.BadArgsf("no subcommand specified")
			}
			return exitcode.BadArgsf("unknown subcommand %q", args[0])
		},
	}

	pf := root.PersistentFlags()
	pf.DurationVar(&flagTimeout, "timeout", 30*time.Second, "how long to wait total")
	pf.DurationVar(&flagInterval, "interval", 500*time.Millisecond, "how often to poll")
	pf.BoolVar(&flagQuiet, "quiet", false, "no output, just exit codes")
	pf.BoolVar(&flagJSON, "json", false, "machine-readable output")
	pf.BoolVar(&flagVerbose, "verbose", false, "show each poll attempt")

	root.AddCommand(
		newPortCmd(),
		newHTTPCmd(),
		newFileCmd(),
		newProcessCmd(),
		newCommandCmd(),
		newAllCmd(),
	)
	root.Version = version.String()

	return root
}

// ExecuteContext runs the CLI with a caller-provided context, so signals
// from main can cancel in-flight polling.
func ExecuteContext(ctx context.Context) error {
	return Normalize(NewRootCmd().ExecuteContext(ctx))
}

// Execute runs the CLI with a background context. Used by tests.
func Execute() error {
	return ExecuteContext(context.Background())
}

// Normalize converts untyped Cobra errors (unknown subcommand, unknown flag,
// bad flag value) into exitcode.BadArgs so callers always see a coded error.
func Normalize(err error) error {
	if err == nil {
		return nil
	}
	if _, ok := errors.AsType[*exitcode.Error](err); ok {
		return err
	}
	return &exitcode.Error{Code: exitcode.BadArgs, Err: err}
}
