package app

import (
	"context"

	adaptercrypto "github.com/bnema/sekeve/internal/adapters/crypto"
	adaptergrpc "github.com/bnema/sekeve/internal/adapters/grpc"
	"github.com/bnema/sekeve/internal/domain/service"
	"github.com/bnema/zerowrap"
)

type ClientApp struct {
	Vault  *service.VaultService
	Sync   *adaptergrpc.Client
	Crypto *adaptercrypto.GPGAdapter
}

func NewClientApp(ctx context.Context, serverAddr string, gpgKeyID string) (*ClientApp, error) {
	log := zerowrap.FromCtx(ctx)
	grpcClient, err := adaptergrpc.NewClient(ctx, serverAddr)
	if err != nil {
		return nil, log.WrapErr(err, "failed to create gRPC client")
	}
	crypto := adaptercrypto.NewGPGAdapter()
	vault := service.NewVaultService(crypto, grpcClient, gpgKeyID)
	return &ClientApp{Vault: vault, Sync: grpcClient, Crypto: crypto}, nil
}

func (a *ClientApp) Close(ctx context.Context) error { return a.Sync.Close(ctx) }
