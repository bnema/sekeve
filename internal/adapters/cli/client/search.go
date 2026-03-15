package client

import (
	"os"
	"strings"

	"github.com/bnema/sekeve/internal/adapters/cli/cliconfig"
	"github.com/bnema/sekeve/internal/adapters/cli/styles"
	"github.com/bnema/sekeve/internal/domain/entity"
	"github.com/spf13/cobra"
)

func NewSearchCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "search <query>",
		Short: "Search entries by name",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			query := strings.ToLower(args[0])

			clientApp, err := cliconfig.ConnectAndAuth(ctx, cliconfig.ServerAddr, cliconfig.GPGKeyID)
			if err != nil {
				styles.RenderError(os.Stderr, err)
				return nil
			}
			defer clientApp.Close(ctx)

			all, err := clientApp.Vault.ListEntries(ctx, entity.EntryTypeUnspecified)
			if err != nil {
				styles.RenderError(os.Stderr, err)
				return nil
			}

			var matched []*entity.Envelope
			for _, e := range all {
				if strings.Contains(strings.ToLower(e.Name), query) {
					matched = append(matched, e)
				}
			}

			if cliconfig.JSONOutput {
				return styles.RenderJSON(os.Stdout, matched)
			}
			styles.RenderTable(os.Stdout, matched)
			return nil
		},
	}
}
