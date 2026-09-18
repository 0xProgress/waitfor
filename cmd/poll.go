package cmd

import (
	"fmt"
	"io"
	"time"

	"github.com/0xProgress/waitfor/internal/exitcode"
	"github.com/0xProgress/waitfor/internal/output"
	"github.com/0xProgress/waitfor/internal/poller"
)

// buildPoller creates a Poller from the global flags. When verbose output
// is enabled (and not suppressed by --quiet or --json), it wires an
// OnAttempt callback that writes one line per poll attempt to out.
func buildPoller(out io.Writer) *poller.Poller {
	p := &poller.Poller{
		Timeout:  flagTimeout,
		Interval: flagInterval,
		Verbose:  flagVerbose,
	}
	if flagVerbose && !flagQuiet && !flagJSON {
		p.OnAttempt = func(n int, state string, elapsed time.Duration, ok bool) {
			output.AttemptLine(out, n, state, elapsed, ok)
		}
	}
	return p
}

// finish renders the result and maps it to an exit-code-bearing error.
// Shared by every subcommand so exit-code semantics stay consistent.
func finish(out io.Writer, result poller.Result) error {
	opts := output.Options{
		Quiet:   flagQuiet,
		JSON:    flagJSON,
		Verbose: flagVerbose,
	}
	if err := output.Render(out, result, opts); err != nil {
		return &exitcode.Error{Code: exitcode.BadArgs, Err: err}
	}
	if result.Success {
		return nil
	}
	if result.Err != nil {
		return exitcode.FromErr(result.Err, exitcode.BadArgs)
	}
	// Timeout or interruption — details already printed.
	return exitcode.SilentTimeout()
}

func printWarnings(out io.Writer, cond any) {
	if flagQuiet || flagJSON {
		return
	}
	w, ok := cond.(poller.Warner)
	if !ok {
		return
	}
	for _, warn := range w.Warnings() {
		fmt.Fprintf(out, "warning: %s\n", warn)
	}
}
