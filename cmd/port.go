package cmd

import (
	"github.com/spf13/cobra"

	"github.com/0xProgress/waitfor/internal/exitcode"
	"github.com/0xProgress/waitfor/internal/network"
)

func newPortCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "port <number>",
		Short: "Wait for a TCP port on localhost to accept connections",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cond, err := network.NewPortCondition(args[0])
			if err != nil {
				return exitcode.BadArgsf("%s", err.Error())
			}

			out := cmd.OutOrStdout()
			p := buildPoller(out)
			result := p.Run(cmd.Context(), cond)
			return finish(out, result)
		},
	}
}
