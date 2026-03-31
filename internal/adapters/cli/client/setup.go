package client

import (
	"fmt"
	"os"

	"github.com/bnema/sekeve/internal/adapters/cli/cliconfig"
	"github.com/bnema/sekeve/internal/adapters/gui"
	"github.com/bnema/sekeve/internal/adapters/notification"
	"github.com/spf13/cobra"
)

// WithGUI wraps a cobra command to wire the GUIAdapter before execution.
// This keeps the gui import in the client package, away from root/server.
func WithGUI(cmd *cobra.Command) *cobra.Command {
	existing := cmd.PreRunE
	cmd.PreRunE = func(cmd *cobra.Command, args []string) error {
		if existing != nil {
			if err := existing(cmd, args); err != nil {
				return err
			}
		}
		ctx := cmd.Context()
		guiAdapter := gui.NewGUIAdapter()
		ctx = cliconfig.WithPINPrompt(ctx, guiAdapter)
		ctx = cliconfig.WithGUI(ctx, guiAdapter)

		notifier, err := notification.NewDBus()
		if err != nil {
			notifier = notification.NewNoop()
		}
		cobra.OnFinalize(func() {
			if err := notifier.Close(); err != nil {
				fmt.Fprintf(os.Stderr, "sekeve: notification cleanup: %v\n", err)
			}
		})
		ctx = cliconfig.WithNotify(ctx, notifier)

		cmd.SetContext(ctx)
		return nil
	}
	return cmd
}
