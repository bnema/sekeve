//go:build linux

package client

import (
	"fmt"

	"github.com/bnema/sekeve/internal/adapters/gui"
	"github.com/spf13/cobra"
)

func NewPINPromptCmd() *cobra.Command {
	var errorMode bool
	var message string

	cmd := &cobra.Command{
		Use:    "pin-prompt",
		Short:  "GUI PIN prompt (internal)",
		Hidden: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			pin, err := gui.RunPINPrompt(errorMode, message)
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
