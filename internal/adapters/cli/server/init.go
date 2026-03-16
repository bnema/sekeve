package server

import (
	"context"

	adaptercrypto "github.com/bnema/sekeve/internal/adapters/crypto"
	"github.com/bnema/sekeve/internal/adapters/storage"
	"github.com/bnema/zerowrap"
	"github.com/spf13/cobra"
)

func NewInitCmd() *cobra.Command {
	var pubKeyFile string
	var dataPath string

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize the server with a client public key",
		Long: `Initialize the server by registering an armored GPG public key.

The key can be provided via:
  --pubkey-file <path>    Read from a file
  stdin pipe              cat key.asc | sekeve server init
  interactive paste       Launches a TUI when no other input is given`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()
			log := zerowrap.FromCtx(ctx)

			rawKey, source, err := readPublicKeyInput(cmd, pubKeyFile)
			if err != nil {
				log.Error().Err(err).Msg("failed to read public key")
				return err
			}

			gpg := adaptercrypto.NewGPGAdapter()
			pubKey, err := gpg.ValidateArmoredPublicKey(ctx, rawKey)
			if err != nil {
				log.Error().Err(err).Msg("invalid public key")
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

			log.Info().Str("source", source).Msg("GPG public key registered successfully")
			return nil
		},
	}

	cmd.Flags().StringVar(&pubKeyFile, "pubkey-file", "", "Path to armored GPG public key file")
	cmd.Flags().StringVar(&dataPath, "data", "./sekeve.db", "Path to bbolt database")
	return cmd
}
