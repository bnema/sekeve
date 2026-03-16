package client

import (
	"os"

	"github.com/bnema/sekeve/internal/adapters/cli/cliconfig"
	"github.com/bnema/sekeve/internal/adapters/cli/styles"
	"github.com/bnema/sekeve/internal/domain/entity"
	"github.com/spf13/cobra"
)

func NewSearchCmd() *cobra.Command {
	var domain, email string

	cmd := &cobra.Command{
		Use:   "search [query]",
		Short: "Search entries by name, domain, or email",
		Long: `Search entries across name, domain/site, and username/email.

Examples:
  sekeve search gmail
  sekeve search --domain gmail.com
  sekeve search --email test@gmail.com`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

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

			all, err := clientApp.Vault.ListEntries(ctx, entity.EntryTypeUnspecified)
			if err != nil {
				_ = styles.RenderError(os.Stderr, err)
				return err
			}

			opts := resolveOpts{Domain: domain, Email: email}
			if len(args) > 0 {
				opts.Query = args[0]
			}
			matched := filterEntries(all, opts)

			if cliconfig.JSONOutput {
				return styles.RenderJSON(os.Stdout, matched)
			}
			return styles.RenderTable(os.Stdout, matched)
		},
	}

	cmd.Flags().StringVar(&domain, "domain", "", "Filter by domain/site")
	cmd.Flags().StringVar(&email, "email", "", "Filter by username/email")
	return cmd
}
