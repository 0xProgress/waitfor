package cmd

import (
	"github.com/spf13/cobra"

	"github.com/0xProgress/waitfor/internal/command"
	"github.com/0xProgress/waitfor/internal/exitcode"
)

func newCommandCmd() *cobra.Command {
	var exitCode int

	c := &cobra.Command{
		Use:   `command "<shell command>"`,
		Short: "Wait for a shell command to exit with the expected code",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cond, err := command.NewCommandCondition(args[0], command.CommandOptions{
				ExitCode: exitCode,
			})
			if err != nil {
				return exitcode.BadArgsf("%s", err.Error())
			}

			out := cmd.OutOrStdout()
			p := buildPoller(out)
			result := p.Run(cmd.Context(), cond)
			return finish(out, result)
		},
	}

	c.Flags().IntVar(&exitCode, "exit-code", 0, "expected exit code")
	return c
}
