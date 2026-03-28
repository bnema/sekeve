//go:build !linux

package client

import "github.com/spf13/cobra"

func NewPINPromptCmd() *cobra.Command {
	return nil
}
