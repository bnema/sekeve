package client

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/bnema/sekeve/internal/adapters/cli/cliconfig"
	"github.com/bnema/sekeve/internal/adapters/cli/styles"
	"github.com/bnema/sekeve/internal/domain/entity"
	"github.com/bnema/zerowrap"
	"github.com/spf13/cobra"
)

func NewDmenuCmd() *cobra.Command {
	var listMode bool
	var copySelection string
	var ensureSession bool

	cmd := &cobra.Command{
		Use:   "dmenu",
		Short: "dmenu/rofi integration for secret picker",
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()
			log := zerowrap.FromCtx(ctx)
			log.Debug().Bool("list", listMode).Str("copy", copySelection).Msg("dmenu invoked")

			cfg := cliconfig.ConfigFromCmd(cmd)
			clientApp, err := cliconfig.ConnectAndAuth(ctx, cfg)
			if err != nil {
				log.Error().Err(err).Msg("connect/auth failed")
				_ = styles.RenderError(os.Stderr, err)
				return err
			}
			log.Debug().Msg("connected and authenticated")
			defer func() {
				if err := clientApp.Close(ctx); err != nil {
					_ = styles.RenderError(os.Stderr, err)
				}
			}()

			if ensureSession {
				return nil
			}

			if listMode {
				entries, err := clientApp.Vault.ListEntries(ctx, entity.EntryTypeUnspecified)
				if err != nil {
					log.Error().Err(err).Msg("list entries failed")
					_ = styles.RenderError(os.Stderr, err)
					return err
				}
				log.Debug().Int("count", len(entries)).Msg("listing entries")
				for _, e := range entries {
					fmt.Println(formatDmenuLine(e))
				}
				return nil
			}

			if copySelection != "" {
				log.Debug().Str("raw", copySelection).Msg("raw selection")
				id := parseDmenuSelection(copySelection)
				log.Debug().Str("id", id).Msg("parsed selection")
				if id == "" {
					err := fmt.Errorf("could not parse selection: %q", copySelection)
					log.Error().Err(err).Msg("parse failed")
					_ = styles.RenderError(os.Stderr, err)
					return err
				}

				env, err := clientApp.Vault.GetEntry(ctx, id)
				if err != nil {
					log.Error().Err(err).Str("id", id).Msg("GetEntry failed")
					_ = styles.RenderError(os.Stderr, err)
					return err
				}
				log.Debug().Str("name", env.Name).Int("type", int(env.Type)).Msg("got entry")

				var copyErr error
				decryptErr := clientApp.Vault.DecryptAndUse(ctx, env.Payload, func(plaintext []byte) {
					var value string
					switch env.Type {
					case entity.EntryTypeLogin:
						var login entity.Login
						if err := json.Unmarshal(plaintext, &login); err != nil {
							copyErr = err
							log.Error().Err(err).Msg("unmarshal login failed")
							return
						}
						value = login.Password
					case entity.EntryTypeSecret:
						var secret entity.Secret
						if err := json.Unmarshal(plaintext, &secret); err != nil {
							copyErr = err
							log.Error().Err(err).Msg("unmarshal secret failed")
							return
						}
						value = secret.Value
					case entity.EntryTypeNote:
						var note entity.Note
						if err := json.Unmarshal(plaintext, &note); err != nil {
							copyErr = err
							log.Error().Err(err).Msg("unmarshal note failed")
							return
						}
						value = note.Content
					default:
						value = string(plaintext)
					}

					log.Debug().Int("len", len(value)).Msg("decrypted, copying to clipboard")
					clipCmd, clipName := clipboardCmd(ctx)
					clipCmd.Stdin = strings.NewReader(value)
					clipCmd.Stderr = os.Stderr
					if err := clipCmd.Run(); err != nil {
						copyErr = fmt.Errorf("%s failed: %w", clipName, err)
						log.Error().Err(err).Str("tool", clipName).Msg("clipboard copy failed")
						return
					}
					log.Debug().Str("tool", clipName).Msg("clipboard copy succeeded")
					_ = exec.CommandContext(ctx, "notify-send", "-a", "sekeve", "-i", "dialog-password", "Sekeve", "Password copied to clipboard").Run()
				})

				if decryptErr != nil {
					log.Error().Err(decryptErr).Msg("decrypt failed")
					_ = styles.RenderError(os.Stderr, decryptErr)
					return decryptErr
				}
				if copyErr != nil {
					log.Error().Err(copyErr).Msg("copy failed")
					_ = styles.RenderError(os.Stderr, copyErr)
					return copyErr
				}
				log.Debug().Msg("dmenu copy done")
				return nil
			}

			return cmd.Help()
		},
	}

	cmd.Flags().BoolVar(&ensureSession, "ensure-session", false, "Authenticate and cache session, then exit")
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

// clipboardCmd returns an exec.Cmd that writes stdin to the system clipboard,
// along with the tool name for logging. Prefers wl-copy on Wayland, falls back
// to xclip on X11.
func clipboardCmd(ctx context.Context) (*exec.Cmd, string) {
	if os.Getenv("WAYLAND_DISPLAY") != "" {
		return exec.CommandContext(ctx, "wl-copy"), "wl-copy"
	}
	return exec.CommandContext(ctx, "xclip", "-selection", "clipboard"), "xclip"
}

func parseDmenuSelection(selection string) string {
	s := strings.TrimSpace(selection)
	// With fuzzel --accept-nth=2, the selection is just the ID.
	// Also handle the full "label\tid" line for other dmenu implementations.
	parts := strings.SplitN(s, "\t", 2)
	if len(parts) == 2 {
		return strings.TrimSpace(parts[1])
	}
	return s
}
