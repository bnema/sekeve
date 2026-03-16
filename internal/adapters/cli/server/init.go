package server

import (
	"context"
	"fmt"

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
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()
			log := zerowrap.FromCtx(ctx)

			if gpgKeyID == "" {
				return fmt.Errorf("--gpg-key is required")
			}

			gpg := adaptercrypto.NewGPGAdapter()
			pubKey, err := gpg.ExportPublicKey(ctx, gpgKeyID)
			if err != nil {
				log.Error().Err(err).Msg("failed to export GPG public key")
				return err
			}

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

			if err := store.StoreAuthKey(ctx, pubKey); err != nil {
				log.Error().Err(err).Msg("failed to store auth key")
				return err
			}

			log.Info().Str("key_id", gpgKeyID).Msg("GPG key registered successfully")
			return nil
		},
	}

	cmd.Flags().StringVar(&gpgKeyID, "gpg-key", "", "GPG key ID to register (required)")
	cmd.Flags().StringVar(&dataPath, "data", "./sekeve.db", "Path to bbolt database")
	if err := cmd.MarkFlagRequired("gpg-key"); err != nil {
		panic(fmt.Sprintf("failed to mark gpg-key flag as required: %v", err))
	}
	return cmd
}
