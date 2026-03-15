package server

import (
	"context"
	"os"

	adaptercrypto "github.com/bnema/sekeve/internal/adapters/crypto"
	"github.com/bnema/sekeve/internal/adapters/storage"
	"github.com/bnema/zerowrap"
	"github.com/spf13/cobra"
)

func NewInitCmd() *cobra.Command {
	var gpgKeyID string
	var dataPath string

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize the server with a GPG key",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			log := zerowrap.FromCtx(ctx)

			if gpgKeyID == "" {
				log.Error().Msg("--gpg-key is required")
				os.Exit(1)
			}

			gpg := adaptercrypto.NewGPGAdapter()
			pubKey, err := gpg.ExportPublicKey(ctx, gpgKeyID)
			if err != nil {
				log.Error().Err(err).Msg("failed to export GPG public key")
				os.Exit(1)
			}

			store, err := storage.NewBboltStore(ctx, dataPath)
			if err != nil {
				log.Error().Err(err).Msg("failed to open storage")
				os.Exit(1)
			}
			defer store.Close(context.Background())

			if err := store.StoreAuthKey(ctx, pubKey); err != nil {
				log.Error().Err(err).Msg("failed to store auth key")
				os.Exit(1)
			}

			log.Info().Str("key_id", gpgKeyID).Msg("GPG key registered successfully")
			return nil
		},
	}

	cmd.Flags().StringVar(&gpgKeyID, "gpg-key", "", "GPG key ID to register (required)")
	cmd.Flags().StringVar(&dataPath, "data", "./sekeve.db", "Path to bbolt database")
	cmd.MarkFlagRequired("gpg-key")
	return cmd
}
