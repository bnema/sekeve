//go:build linux

package client

import (
	"errors"
	"fmt"
	"os"

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
			pin, err := gui.RunPINPrompt(errorMode, message)
			if errors.Is(err, gui.ErrCancelled) {
				os.Exit(1)
			}
			if err != nil {
				return err
			}
			fmt.Println(pin)
			return nil
		},
	}

	cmd.Flags().BoolVar(&errorMode, "error", false, "Show error styling")
	cmd.Flags().StringVar(&message, "message", "", "Message to display above input")
	return cmd
}
