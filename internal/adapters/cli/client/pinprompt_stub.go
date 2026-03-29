//go:build !linux || !gui

package client

import "github.com/spf13/cobra"

func NewPINPromptCmd() *cobra.Command {
	return nil
}
