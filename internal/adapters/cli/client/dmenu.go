package client

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/bnema/sekeve/internal/adapters/cli/cliconfig"
	"github.com/bnema/sekeve/internal/adapters/cli/styles"
	"github.com/bnema/sekeve/internal/domain/entity"
	"github.com/spf13/cobra"
)

func NewDmenuCmd() *cobra.Command {
	var listMode bool
	var copySelection string

	cmd := &cobra.Command{
		Use:   "dmenu",
		Short: "dmenu/rofi integration for secret picker",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			clientApp, err := cliconfig.ConnectAndAuth(ctx, cliconfig.ServerAddr, cliconfig.GPGKeyID)
			if err != nil {
				styles.RenderError(os.Stderr, err)
				return err
			}
			defer clientApp.Close(ctx)

			if listMode {
				entries, err := clientApp.Vault.ListEntries(ctx, entity.EntryTypeUnspecified)
				if err != nil {
					styles.RenderError(os.Stderr, err)
					return err
				}
				for _, e := range entries {
					fmt.Println(formatDmenuLine(e))
				}
				return nil
			}

			if copySelection != "" {
				name := parseDmenuSelection(copySelection)
				if name == "" {
					err := fmt.Errorf("could not parse selection: %q", copySelection)
					styles.RenderError(os.Stderr, err)
					return err
				}

				env, err := clientApp.Vault.GetEntry(ctx, name)
				if err != nil {
					styles.RenderError(os.Stderr, err)
					return err
				}

				var copyErr error
				decryptErr := clientApp.Vault.DecryptAndUse(ctx, env.Payload, func(plaintext []byte) {
					var value string
					switch env.Type {
					case entity.EntryTypeLogin:
						var login entity.Login
						if err := json.Unmarshal(plaintext, &login); err != nil {
							copyErr = err
							return
						}
						value = login.Password
					case entity.EntryTypeSecret:
						var secret entity.Secret
						if err := json.Unmarshal(plaintext, &secret); err != nil {
							copyErr = err
							return
						}
						value = secret.Value
					case entity.EntryTypeNote:
						var note entity.Note
						if err := json.Unmarshal(plaintext, &note); err != nil {
							copyErr = err
							return
						}
						value = note.Content
					default:
						value = string(plaintext)
					}

					wlCopy := exec.CommandContext(ctx, "wl-copy")
					wlCopy.Stdin = strings.NewReader(value)
					wlCopy.Stderr = os.Stderr
					if err := wlCopy.Run(); err != nil {
						copyErr = fmt.Errorf("wl-copy failed: %w", err)
					}
				})

				if decryptErr != nil {
					styles.RenderError(os.Stderr, decryptErr)
					return decryptErr
				}
				if copyErr != nil {
					styles.RenderError(os.Stderr, copyErr)
					return copyErr
				}
				return nil
			}

			cmd.Help()
			return nil
		},
	}

	cmd.Flags().BoolVar(&listMode, "list", false, "List entries for dmenu input")
	cmd.Flags().StringVar(&copySelection, "copy", "", "Copy value for selected dmenu entry")
	return cmd
}

func formatDmenuLine(e *entity.Envelope) string {
	switch e.Type {
	case entity.EntryTypeLogin:
		site := e.Meta["site"]
		username := e.Meta["username"]
		if site != "" {
			return fmt.Sprintf("🔑 %s · %s · %s", e.Name, site, username)
		}
		return fmt.Sprintf("🔑 %s · %s", e.Name, username)
	case entity.EntryTypeSecret:
		return fmt.Sprintf("🔒 %s", e.Name)
	case entity.EntryTypeNote:
		return fmt.Sprintf("📝 %s", e.Name)
	default:
		return e.Name
	}
}

func parseDmenuSelection(selection string) string {
	// Strip emoji prefix: find first space after any leading non-letter rune
	s := strings.TrimSpace(selection)
	// Remove leading emoji and space
	for len(s) > 0 {
		r := []rune(s)[0]
		// emoji code points are typically > 0x2000
		if r > 0x2000 || r == ' ' {
			s = strings.TrimSpace(string([]rune(s)[1:]))
		} else {
			break
		}
	}
	// Take text before first " · "
	parts := strings.SplitN(s, " · ", 2)
	return strings.TrimSpace(parts[0])
}
