package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"

	"github.com/0xProgress/waitfor/cmd"
	"github.com/0xProgress/waitfor/internal/exitcode"
)

func main() {
	ctx, stop := signal.NotifyContext(
		context.Background(),
		os.Interrupt,
		syscall.SIGTERM,
	)
	defer stop()

	os.Exit(runContext(ctx, os.Args[1:], os.Stdout, os.Stderr))
}

// runContext runs the CLI with an explicit context, args, and writers, and
// returns the process exit code. It is the testable core of main; main
// supplies os.Args, os.Stdout/os.Stderr, and a signal-aware context.
func runContext(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	root := cmd.NewRootCmd()
	root.SetArgs(args)
	root.SetOut(stdout)
	root.SetErr(stderr)

	err := cmd.Normalize(root.ExecuteContext(ctx))
	if err == nil {
		return exitcode.Success
	}

	if ce, ok := errors.AsType[*exitcode.Error](err); ok {
		if !ce.Silent && ce.Error() != "" {
			fmt.Fprintln(stderr, "waitfor: "+ce.Error())
		}
		return ce.Code
	}

	fmt.Fprintln(stderr, "waitfor: "+err.Error())
	return exitcode.BadArgs
}
