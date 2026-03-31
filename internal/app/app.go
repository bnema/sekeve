package app

import (
	"context"

	adaptergrpc "github.com/bnema/sekeve/internal/adapters/grpc"
	"github.com/bnema/sekeve/internal/domain/service"
	"github.com/bnema/sekeve/internal/port"
	"github.com/bnema/zerowrap"
)

type ClientApp struct {
	Vault  *service.VaultService
	Sync   *adaptergrpc.Client
	Config port.ConfigPort
}

func NewClientApp(ctx context.Context, cfg port.ConfigPort, crypto port.CryptoPort) (*ClientApp, error) {
	log := zerowrap.FromCtx(ctx)

	serverAddr := cfg.ServerAddr(ctx)
	gpgKeyID := cfg.GPGKeyID(ctx)

	grpcClient, err := adaptergrpc.NewClient(ctx, serverAddr)
	if err != nil {
		return nil, log.WrapErr(err, "failed to create gRPC client")
	}
	vault := service.NewVaultService(crypto, grpcClient, gpgKeyID)
	return &ClientApp{Vault: vault, Sync: grpcClient, Config: cfg}, nil
}

func (a *ClientApp) Close(ctx context.Context) error { return a.Sync.Close(ctx) }
