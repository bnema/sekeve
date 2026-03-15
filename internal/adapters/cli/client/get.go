package client

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/bnema/sekeve/internal/adapters/cli/cliconfig"
	"github.com/bnema/sekeve/internal/adapters/cli/styles"
	"github.com/bnema/sekeve/internal/domain/entity"
	"github.com/spf13/cobra"
)

func NewGetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get <name>",
		Short: "Get and decrypt an entry",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			name := args[0]

			clientApp, err := cliconfig.ConnectAndAuth(ctx, cliconfig.ServerAddr, cliconfig.GPGKeyID)
			if err != nil {
				styles.RenderError(os.Stderr, err)
				return nil
			}
			defer clientApp.Close(ctx)

			env, err := clientApp.Vault.GetEntry(ctx, name)
			if err != nil {
				styles.RenderError(os.Stderr, err)
				return nil
			}

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
					fields := map[string]string{
						"Username": login.Username,
						"Password": login.Password,
					}
					if login.Site != "" {
						fields["Site"] = login.Site
					}
					if login.Notes != "" {
						fields["Notes"] = login.Notes
					}
					styles.RenderEntry(os.Stdout, env, fields)

				case entity.EntryTypeSecret:
					var secret entity.Secret
					if err := json.Unmarshal(plaintext, &secret); err != nil {
						displayErr = err
						return
					}
					styles.RenderEntry(os.Stdout, env, map[string]string{
						"Value": secret.Value,
					})

				case entity.EntryTypeNote:
					var note entity.Note
					if err := json.Unmarshal(plaintext, &note); err != nil {
						displayErr = err
						return
					}
					styles.RenderEntry(os.Stdout, env, map[string]string{
						"Content": note.Content,
					})

				default:
					fmt.Fprintf(os.Stdout, "%s\n", string(plaintext))
				}
			})
			if decryptErr != nil {
				styles.RenderError(os.Stderr, decryptErr)
				return nil
			}
			if displayErr != nil {
				styles.RenderError(os.Stderr, displayErr)
			}
			return nil
		},
	}
}
