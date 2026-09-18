package cmd

import (
	"github.com/spf13/cobra"

	"github.com/0xProgress/waitfor/internal/exitcode"
	"github.com/0xProgress/waitfor/internal/process"
)

func newProcessCmd() *cobra.Command {
	var (
		exact    bool
		userFlag string
	)

	c := &cobra.Command{
		Use:   "process <name>",
		Short: "Wait for a process with the given name to be running",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cond, err := process.NewProcessCondition(args[0], process.ProcessOptions{
				Exact: exact,
				User:  userFlag,
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

	c.Flags().BoolVar(&exact, "exact", false, "exact name match instead of substring")
	c.Flags().StringVar(&userFlag, "user", "", "only match processes owned by this user")
	return c
}
