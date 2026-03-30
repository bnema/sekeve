package client

import (
	"github.com/bnema/sekeve/internal/adapters/cli/cliconfig"
	"github.com/bnema/sekeve/internal/adapters/gui"
	"github.com/spf13/cobra"
)

// WithGUI wraps a cobra command to wire the GUIAdapter before execution.
// This keeps the gui import in the client package, away from root/server.
func WithGUI(cmd *cobra.Command) *cobra.Command {
	existing := cmd.PersistentPreRunE
	cmd.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
		if existing != nil {
			if err := existing(cmd, args); err != nil {
				return err
			}
		}
		ctx := cmd.Context()
		guiAdapter := gui.NewGUIAdapter()
		ctx = cliconfig.WithPINPrompt(ctx, guiAdapter)
		ctx = cliconfig.WithGUI(ctx, guiAdapter)
		cmd.SetContext(ctx)
		return nil
	}
	return cmd
}
