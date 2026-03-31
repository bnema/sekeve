package server

import (
	"context"
	"errors"
	"fmt"

	adaptercrypto "github.com/bnema/sekeve/internal/adapters/crypto"
	grpcadapter "github.com/bnema/sekeve/internal/adapters/grpc"
	"github.com/bnema/sekeve/internal/adapters/storage"
	"github.com/bnema/sekeve/internal/port"
	"github.com/bnema/zerowrap"
	"github.com/spf13/cobra"
)

func NewStartCmd() *cobra.Command {
	var addr string
	var dataPath string

	cmd := &cobra.Command{
		Use:   "start",
		Short: "Start the sekeve gRPC server",
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()
			log := zerowrap.FromCtx(ctx)

			store, err := storage.NewBboltStore(ctx, dataPath)
			if err != nil {
				log.Error().Err(err).Msg("failed to open storage")
				return err
			}
			defer func() {
				if err := store.Close(context.Background()); err != nil {
					log.Error().Err(err).Msg("failed to close store")
				}
			}()

			pubKey, err := store.GetAuthKey(ctx)
			if err != nil {
				if errors.Is(err, port.ErrNotFound) {
					return fmt.Errorf("no public key registered; run 'sekeve server init' first")
				}
				log.Error().Err(err).Msg("failed to load auth key")
				return err
			}

			authManager := grpcadapter.NewAuthManager(pubKey)
			crypto := adaptercrypto.NewGPGAdapter()
			server := grpcadapter.NewServer(ctx, store, authManager, crypto)

			log.Info().Str("addr", addr).Msg("starting gRPC server")
			if err := server.Serve(ctx, addr); err != nil {
				log.Error().Err(err).Msg("server error")
				return err
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&addr, "addr", ":50051", "Address to listen on")
	cmd.Flags().StringVar(&dataPath, "data", "./sekeve.db", "Path to bbolt database")
	return cmd
}
