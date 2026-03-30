package client

import (
	"os"

	"github.com/bnema/sekeve/internal/adapters/cli/cliconfig"
	"github.com/bnema/sekeve/internal/adapters/cli/styles"
	"github.com/bnema/sekeve/internal/domain/entity"
	"github.com/bnema/sekeve/internal/port"
	"github.com/bnema/zerowrap"
	"github.com/spf13/cobra"
)

// NewOmniboxCmd creates the `sekeve omnibox` command.
func NewOmniboxCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "omnibox",
		Short: "Open the sekeve omnibox overlay",
		Long: `Opens the GTK4 omnibox overlay for searching, adding, and editing entries.
If no active session exists, a PIN prompt is shown first.

Bind this to a keyboard shortcut (e.g. Ctrl+Super+P) in your compositor.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()
			log := zerowrap.FromCtx(ctx)

			cfg := cliconfig.ConfigFromCmd(cmd)

			// Connect and authenticate (shows PIN prompt if needed).
			clientApp, err := cliconfig.ConnectAndAuth(ctx, cfg)
			if err != nil {
				log.Error().Err(err).Msg("connect/auth failed")
				_ = styles.RenderError(os.Stderr, err)
				return err
			}
			defer func() {
				if closeErr := clientApp.Close(ctx); closeErr != nil {
					_ = styles.RenderError(os.Stderr, closeErr)
				}
			}()

			// Show omnibox (defaults to Search / All).
			guiPort := cliconfig.GUIFromCtx(ctx)
			omniCfg := port.OmniboxConfig{
				Mode:     port.OmniboxModeSearch,
				Category: entity.EntryTypeUnspecified, // All
			}

			if err := guiPort.ShowOmnibox(ctx, omniCfg); err != nil {
				log.Error().Err(err).Msg("omnibox failed")
				_ = styles.RenderError(os.Stderr, err)
				return err
			}

			return nil
		},
	}
}
