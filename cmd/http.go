package cmd

import (
	"github.com/spf13/cobra"

	"github.com/0xProgress/waitfor/internal/exitcode"
	"github.com/0xProgress/waitfor/internal/network"
)

func newHTTPCmd() *cobra.Command {
	var (
		status   int
		method   string
		insecure bool
		headers  []string
	)

	c := &cobra.Command{
		Use:   "http <url>",
		Short: "Wait for an HTTP endpoint to return the expected status",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cond, err := network.NewHTTPCondition(args[0], network.HTTPOptions{
				Method:   method,
				WantCode: status,
				Insecure: insecure,
				Headers:  headers,
			})
			if err != nil {
				return exitcode.BadArgsf("%s", err.Error())
			}

			out := cmd.OutOrStdout()
			printWarnings(out, cond)
			p := buildPoller(out)
			result := p.Run(cmd.Context(), cond)
			return finish(out, result)
		},
	}

	c.Flags().IntVar(&status, "status", 0, "expected status code (default: any 2xx)")
	c.Flags().StringVar(&method, "method", "GET", "HTTP method")
	c.Flags().BoolVar(&insecure, "insecure", false, "skip TLS verification")
	c.Flags().StringArrayVar(&headers, "header", nil, `add request header "Key: Value"`)
	return c
}
