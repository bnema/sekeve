package cli

import (
	"context"

	"github.com/bnema/sekeve/internal/adapters/cli/cliconfig"
	"github.com/bnema/sekeve/internal/app"
	"github.com/bnema/sekeve/internal/port"
)

func ConnectAndAuth(ctx context.Context, cfg port.ConfigPort) (*app.ClientApp, error) {
	return cliconfig.ConnectAndAuth(ctx, cfg)
}
