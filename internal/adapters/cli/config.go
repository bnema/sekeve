// Package cli provides re-exports of cliconfig helpers for backward compatibility.
package cli

import (
	"context"

	"github.com/bnema/sekeve/internal/adapters/cli/cliconfig"
	"github.com/bnema/sekeve/internal/port"
	"github.com/spf13/cobra"
)

func ConfigFromCmd(cmd *cobra.Command) port.ConfigPort { return cliconfig.ConfigFromCmd(cmd) }
func WithConfig(ctx context.Context, cfg port.ConfigPort) context.Context {
	return cliconfig.WithConfig(ctx, cfg)
}
