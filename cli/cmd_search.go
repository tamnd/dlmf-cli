package cli

import (
	"github.com/spf13/cobra"
)

func (a *App) searchCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "search <query>",
		Short: "Search the DLMF for mathematical functions and topics",
		Long: `Search the NIST DLMF for sections matching a query.
Results include the section number, title, and URL.

Examples:
  dlmf search "bessel function"
  dlmf search "gamma function" --limit 5 -o json
  dlmf search "hypergeometric" -o url`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			n := a.effectiveLimit(10)
			a.progressf("searching for %q...", args[0])
			results, err := a.client.Search(cmd.Context(), args[0], n)
			if err != nil {
				return mapFetchErr(err)
			}
			return a.renderOrEmpty(results, len(results))
		},
	}
}
