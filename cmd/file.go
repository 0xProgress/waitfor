package cmd

import (
	"github.com/spf13/cobra"

	"github.com/0xProgress/waitfor/internal/exitcode"
	"github.com/0xProgress/waitfor/internal/file"
)

func newFileCmd() *cobra.Command {
	var (
		minSize  int64
		nonEmpty bool
	)

	c := &cobra.Command{
		Use:   "file <path>",
		Short: "Wait for a file or directory to exist",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cond, err := file.NewFileCondition(args[0], file.FileOptions{
				MinSize:  minSize,
				NonEmpty: nonEmpty,
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

	c.Flags().Int64Var(&minSize, "min-size", 0, "file must be at least N bytes")
	c.Flags().BoolVar(&nonEmpty, "nonempty", false, "shorthand for --min-size 1")
	return c
}
