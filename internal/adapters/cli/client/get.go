package client

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/bnema/sekeve/internal/adapters/cli/cliconfig"
	"github.com/bnema/sekeve/internal/adapters/cli/styles"
	"github.com/bnema/sekeve/internal/app"
	"github.com/bnema/sekeve/internal/domain/entity"
	"github.com/spf13/cobra"
)

func NewGetCmd() *cobra.Command {
	var id, domain, email string

	cmd := &cobra.Command{
		Use:   "get [query]",
		Short: "Get and decrypt an entry",
		Long: `Search for an entry and decrypt it. Searches across name, domain, and username.

Examples:
  sekeve get gmail                     # fuzzy search
  sekeve get --domain gmail.com        # filter by domain
  sekeve get --email test@gmail.com    # filter by username/email
  sekeve get --id <uuid>               # direct fetch by ID`,
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

			if opts.ID != "" {
				env, err := clientApp.Vault.GetEntry(ctx, opts.ID)
				if err != nil {
					_ = styles.RenderError(os.Stderr, err)
					return err
				}
				return displayDecryptedEntry(clientApp, ctx, env)
			}

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

			env, err = clientApp.Vault.GetEntry(ctx, env.ID)
			if err != nil {
				_ = styles.RenderError(os.Stderr, err)
				return err
			}

			return displayDecryptedEntry(clientApp, ctx, env)
		},
	}

	cmd.Flags().StringVar(&id, "id", "", "Get entry by exact ID")
	cmd.Flags().StringVar(&domain, "domain", "", "Filter by domain/site")
	cmd.Flags().StringVar(&email, "email", "", "Filter by username/email")
	return cmd
}

func displayDecryptedEntry(clientApp *app.ClientApp, ctx context.Context, env *entity.Envelope) error {
	var displayErr error
	decryptErr := clientApp.Vault.DecryptAndUse(ctx, env.Payload, func(plaintext []byte) {
		if cliconfig.JSONOutput {
			env.Payload = plaintext
			displayErr = styles.RenderJSON(os.Stdout, env)
			return
		}

		switch env.Type {
		case entity.EntryTypeLogin:
			var login entity.Login
			if err := json.Unmarshal(plaintext, &login); err != nil {
				displayErr = err
				return
			}
			fields := []styles.Field{
				{Label: "Username", Value: login.Username},
				{Label: "Password", Value: login.Password},
			}
			if login.Site != "" {
				fields = append(fields, styles.Field{Label: "Site", Value: login.Site})
			}
			if login.Notes != "" {
				fields = append(fields, styles.Field{Label: "Notes", Value: login.Notes})
			}
			displayErr = styles.RenderEntry(os.Stdout, env, fields)

		case entity.EntryTypeSecret:
			var secret entity.Secret
			if err := json.Unmarshal(plaintext, &secret); err != nil {
				displayErr = err
				return
			}
			displayErr = styles.RenderEntry(os.Stdout, env, []styles.Field{{Label: "Value", Value: secret.Value}})

		case entity.EntryTypeNote:
			var note entity.Note
			if err := json.Unmarshal(plaintext, &note); err != nil {
				displayErr = err
				return
			}
			displayErr = styles.RenderEntry(os.Stdout, env, []styles.Field{{Label: "Content", Value: note.Content}})

		default:
			if _, err := fmt.Fprintf(os.Stdout, "%s\n", string(plaintext)); err != nil {
				displayErr = err
			}
		}
	})
	if decryptErr != nil {
		_ = styles.RenderError(os.Stderr, decryptErr)
		return decryptErr
	}
	if displayErr != nil {
		_ = styles.RenderError(os.Stderr, displayErr)
		return displayErr
	}
	return nil
}
