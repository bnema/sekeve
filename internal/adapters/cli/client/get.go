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

			env, err := clientApp.Vault.GetEntry(ctx, name)
			if err != nil {
				_ = styles.RenderError(os.Stderr, err)
				return err
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
					displayErr = styles.RenderEntry(os.Stdout, env, []styles.Field{
						{Label: "Value", Value: secret.Value},
					})

				case entity.EntryTypeNote:
					var note entity.Note
					if err := json.Unmarshal(plaintext, &note); err != nil {
						displayErr = err
						return
					}
					displayErr = styles.RenderEntry(os.Stdout, env, []styles.Field{
						{Label: "Content", Value: note.Content},
					})

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
		},
	}
}
