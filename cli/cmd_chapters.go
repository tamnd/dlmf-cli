package cli

import (
	"github.com/spf13/cobra"
)

func (a *App) chaptersCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "chapters",
		Short: "List all DLMF chapters",
		Long: `List all 36 chapters of the NIST Digital Library of Mathematical Functions.
Each record includes the chapter number, title, and URL.`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			n := a.effectiveLimit(36)
			a.progressf("fetching chapter list...")
			chapters, err := a.client.Chapters(cmd.Context(), n)
			if err != nil {
				return mapFetchErr(err)
			}
			return a.renderOrEmpty(chapters, len(chapters))
		},
	}
}
