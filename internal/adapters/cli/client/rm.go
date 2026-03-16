package client

import (
	"fmt"
	"os"

	"github.com/bnema/sekeve/internal/adapters/cli/cliconfig"
	"github.com/bnema/sekeve/internal/adapters/cli/styles"
	"github.com/bnema/sekeve/internal/domain/entity"
	"github.com/spf13/cobra"
)

func NewRmCmd() *cobra.Command {
	var id, domain, email string

	cmd := &cobra.Command{
		Use:   "rm [query]",
		Short: "Delete an entry",
		Long: `Search for an entry and delete it.

Examples:
  sekeve rm gmail
  sekeve rm --domain gmail.com
  sekeve rm --email test@gmail.com
  sekeve rm --id <uuid>`,
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

			opts := resolveOpts{ID: id, Domain: domain, Email: email}
			if len(args) > 0 {
				opts.Query = args[0]
			}

			var targetID, targetName string
			if opts.ID != "" {
				targetID = opts.ID
				targetName = opts.ID
			} else {
				if opts.Query == "" && opts.Domain == "" && opts.Email == "" {
					return fmt.Errorf("provide a search query, --domain, --email, or --id")
				}

				all, err := clientApp.Vault.ListEntries(ctx, entity.EntryTypeUnspecified)
				if err != nil {
					_ = styles.RenderError(os.Stderr, err)
					return err
				}

				env, err := resolveEntry(all, opts)
				if err != nil {
					_ = styles.RenderError(os.Stderr, err)
					return err
				}

				targetID = env.ID
				targetName = env.Name
			}

			if err := clientApp.Vault.DeleteEntry(ctx, targetID); err != nil {
				_ = styles.RenderError(os.Stderr, err)
				return err
			}

			if cliconfig.JSONOutput {
				return styles.RenderJSON(os.Stdout, map[string]string{"deleted": targetID})
			}
			return styles.RenderSuccess(os.Stdout, fmt.Sprintf("Entry %q deleted", targetName))
		},
	}

	cmd.Flags().StringVar(&id, "id", "", "Delete entry by exact ID")
	cmd.Flags().StringVar(&domain, "domain", "", "Filter by domain/site")
	cmd.Flags().StringVar(&email, "email", "", "Filter by username/email")
	return cmd
}
