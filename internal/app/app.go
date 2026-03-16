package app

import (
	"context"

	adaptercrypto "github.com/bnema/sekeve/internal/adapters/crypto"
	adaptergrpc "github.com/bnema/sekeve/internal/adapters/grpc"
	"github.com/bnema/sekeve/internal/domain/service"
	"github.com/bnema/sekeve/internal/port"
	"github.com/bnema/zerowrap"
)

type ClientApp struct {
	Vault  *service.VaultService
	Sync   *adaptergrpc.Client
	Crypto *adaptercrypto.GPGAdapter
	Config port.ConfigPort
}

func NewClientApp(ctx context.Context, cfg port.ConfigPort) (*ClientApp, error) {
	log := zerowrap.FromCtx(ctx)

	serverAddr := cfg.ServerAddr(ctx)
	gpgKeyID := cfg.GPGKeyID(ctx)

	grpcClient, err := adaptergrpc.NewClient(ctx, serverAddr)
	if err != nil {
		return nil, log.WrapErr(err, "failed to create gRPC client")
	}
	crypto := adaptercrypto.NewGPGAdapter()
	vault := service.NewVaultService(crypto, grpcClient, gpgKeyID)
	return &ClientApp{Vault: vault, Sync: grpcClient, Crypto: crypto, Config: cfg}, nil
}

func (a *ClientApp) Close(ctx context.Context) error { return a.Sync.Close(ctx) }
