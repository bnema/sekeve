package client

import (
	"fmt"
	"os"

	"github.com/bnema/sekeve/internal/adapters/cli/cliconfig"
	"github.com/bnema/sekeve/internal/adapters/cli/styles"
	"github.com/spf13/cobra"
)

func NewRmCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "rm <name>",
		Short: "Delete an entry",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			name := args[0]

			clientApp, err := cliconfig.ConnectAndAuth(ctx, cliconfig.ServerAddr, cliconfig.GPGKeyID)
			if err != nil {
				styles.RenderError(os.Stderr, err)
				return err
			}
			defer clientApp.Close(ctx)

			if err := clientApp.Vault.DeleteEntry(ctx, name); err != nil {
				styles.RenderError(os.Stderr, err)
				return err
			}

			if cliconfig.JSONOutput {
				return styles.RenderJSON(os.Stdout, map[string]string{"deleted": name})
			}
			styles.RenderSuccess(os.Stdout, fmt.Sprintf("Entry %q deleted", name))
			return nil
		},
	}
}
