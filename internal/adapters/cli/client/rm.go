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

			cfg := cliconfig.ConfigFromCmd(cmd)
			clientApp, err := cliconfig.ConnectAndAuth(ctx, cfg)
			if err != nil {
				_ = styles.RenderError(os.Stderr, err)
				return err
			}
			defer func() {
				if err := clientApp.Close(ctx); err != nil {
					_ = styles.RenderError(os.Stderr, err)
				}
			}()

			if err := clientApp.Vault.DeleteEntry(ctx, name); err != nil {
				_ = styles.RenderError(os.Stderr, err)
				return err
			}

			if cliconfig.JSONOutput {
				return styles.RenderJSON(os.Stdout, map[string]string{"deleted": name})
			}
			return styles.RenderSuccess(os.Stdout, fmt.Sprintf("Entry %q deleted", name))
		},
	}
}
