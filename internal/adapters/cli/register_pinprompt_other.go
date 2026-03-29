//go:build !linux || !gui

package cli

import "github.com/spf13/cobra"

func registerPINPrompt(_ *cobra.Command) {}
