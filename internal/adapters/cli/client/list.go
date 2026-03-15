package client

import (
	"os"

	"github.com/bnema/sekeve/internal/adapters/cli/cliconfig"
	"github.com/bnema/sekeve/internal/adapters/cli/styles"
	"github.com/bnema/sekeve/internal/domain/entity"
	"github.com/spf13/cobra"
)

func NewListCmd() *cobra.Command {
	var entryTypeStr string

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all entries",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			clientApp, err := cliconfig.ConnectAndAuth(ctx, cliconfig.ServerAddr, cliconfig.GPGKeyID)
			if err != nil {
				styles.RenderError(os.Stderr, err)
				return nil
			}
			defer clientApp.Close(ctx)

			entryType := entity.ParseEntryType(entryTypeStr)
			entries, err := clientApp.Vault.ListEntries(ctx, entryType)
			if err != nil {
				styles.RenderError(os.Stderr, err)
				return nil
			}

			if cliconfig.JSONOutput {
				return styles.RenderJSON(os.Stdout, entries)
			}
			styles.RenderTable(os.Stdout, entries)
			return nil
		},
	}

	cmd.Flags().StringVar(&entryTypeStr, "type", "", "Filter by type (login, secret, note)")
	return cmd
}
