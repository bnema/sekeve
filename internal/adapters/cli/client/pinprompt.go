//go:build linux

package client

import (
	"github.com/bnema/sekeve/internal/adapters/gui"
	"github.com/spf13/cobra"
)

func NewPINPromptCmd() *cobra.Command {
	var errorMode bool
	var message string

	cmd := &cobra.Command{
		Use:          "pin-prompt",
		Short:        "GUI PIN prompt (internal)",
		Hidden:       true,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			// RunPINPrompt handles stdout output and os.Exit internally
			// because app.Main() does not return on Wayland.
			_, err := gui.RunPINPrompt(errorMode, message)
			return err
		},
	}

	cmd.Flags().BoolVar(&errorMode, "error", false, "Show error styling")
	cmd.Flags().StringVar(&message, "message", "", "Message to display above input")
	return cmd
}
