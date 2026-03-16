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
		RunE: func(cmd *cobra.Command, _ []string) error {
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

			if listMode {
				entries, err := clientApp.Vault.ListEntries(ctx, entity.EntryTypeUnspecified)
				if err != nil {
					_ = styles.RenderError(os.Stderr, err)
					return err
				}
				for _, e := range entries {
					fmt.Println(formatDmenuLine(e))
				}
				return nil
			}

			if copySelection != "" {
				id := parseDmenuSelection(copySelection)
				if id == "" {
					err := fmt.Errorf("could not parse selection: %q", copySelection)
					_ = styles.RenderError(os.Stderr, err)
					return err
				}

				env, err := clientApp.Vault.GetEntry(ctx, id)
				if err != nil {
					_ = styles.RenderError(os.Stderr, err)
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
					_ = styles.RenderError(os.Stderr, decryptErr)
					return decryptErr
				}
				if copyErr != nil {
					_ = styles.RenderError(os.Stderr, copyErr)
					return copyErr
				}
				return nil
			}

			return cmd.Help()
		},
	}

	cmd.Flags().BoolVar(&listMode, "list", false, "List entries for dmenu input")
	cmd.Flags().StringVar(&copySelection, "copy", "", "Copy value for selected dmenu entry")
	return cmd
}

func formatDmenuLine(e *entity.Envelope) string {
	switch e.Type {
	case entity.EntryTypeLogin:
		return fmt.Sprintf("🔑 %s\t%s", e.Name, e.ID)
	case entity.EntryTypeSecret:
		return fmt.Sprintf("🔒 %s\t%s", e.Name, e.ID)
	case entity.EntryTypeNote:
		return fmt.Sprintf("📝 %s\t%s", e.Name, e.ID)
	default:
		return fmt.Sprintf("%s\t%s", e.Name, e.ID)
	}
}

func parseDmenuSelection(selection string) string {
	s := strings.TrimSpace(selection)
	parts := strings.SplitN(s, "\t", 2)
	if len(parts) == 2 {
		return strings.TrimSpace(parts[1])
	}

	for len(s) > 0 {
		r := []rune(s)[0]
		if r > 0x2000 || r == ' ' {
			s = strings.TrimSpace(string([]rune(s)[1:]))
		} else {
			break
		}
	}
	// Fallback for non-tab-separated input: best effort only. The primary path is
	// "label<TAB>id"; this fallback returns the cleaned token unchanged so pasted
	// IDs still work, and server-side lookup returns a clear error if it is not an ID.
	return strings.TrimSpace(s)
}
