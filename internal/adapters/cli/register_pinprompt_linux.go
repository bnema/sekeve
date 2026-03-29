//go:build linux && gui

package cli

import (
	"github.com/bnema/sekeve/internal/adapters/cli/client"
	"github.com/spf13/cobra"
)

func registerPINPrompt(root *cobra.Command) {
	if cmd := client.NewPINPromptCmd(); cmd != nil {
		root.AddCommand(cmd)
	}
}
