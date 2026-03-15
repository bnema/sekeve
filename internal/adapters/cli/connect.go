package cli

import (
	"context"

	"github.com/bnema/sekeve/internal/adapters/cli/cliconfig"
	"github.com/bnema/sekeve/internal/app"
)

func ConnectAndAuth(ctx context.Context, serverAddr, gpgKeyID string) (*app.ClientApp, error) {
	return cliconfig.ConnectAndAuth(ctx, serverAddr, gpgKeyID)
}
