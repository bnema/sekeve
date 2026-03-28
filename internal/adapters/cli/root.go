package cli

import (
	"context"
	"os"
	"os/signal"

	"fmt"

	"github.com/bnema/sekeve/internal/adapters/cli/cliconfig"
	"github.com/bnema/sekeve/internal/adapters/cli/client"
	"github.com/bnema/sekeve/internal/adapters/cli/server"
	adapterconfig "github.com/bnema/sekeve/internal/adapters/config"
	logadapter "github.com/bnema/sekeve/internal/adapters/logger"
	"github.com/bnema/sekeve/internal/adapters/xdg"
	"github.com/bnema/sekeve/internal/version"
	"github.com/spf13/cobra"
)

func NewRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:     "sekeve",
		Short:   "Sekeve - CLI secret manager with GPG encryption",
		Version: version.Version,
		PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
			ctx, cancel := signal.NotifyContext(cmd.Context(), os.Interrupt)
			_, ctx = logadapter.New(ctx)
			cobra.OnFinalize(func() { cancel() })

			xdgAdapter := xdg.NewAdapter("sekeve")
			cfg, err := adapterconfig.NewViperConfig(ctx, xdgAdapter)
			if err != nil {
				return err
			}

			// CLI flags override config file values
			if cliconfig.ServerAddr != "" {
				cfg.SetOverride("server_addr", cliconfig.ServerAddr)
			}
			if cliconfig.GPGKeyID != "" {
				cfg.SetOverride("gpg_key_id", cliconfig.GPGKeyID)
			}

			ctx = cliconfig.WithConfig(ctx, cfg)
			cmd.SetContext(ctx)
			return nil
		},
	}

	root.PersistentFlags().StringVar(&cliconfig.ServerAddr, "server", "", "server address")
	root.PersistentFlags().StringVar(&cliconfig.GPGKeyID, "gpg-key", "", "GPG key ID")
	root.PersistentFlags().BoolVar(&cliconfig.JSONOutput, "json", false, "output as JSON")

	root.SetVersionTemplate(fmt.Sprintf("Sekeve %s\n", version.Version))

	root.AddCommand(&cobra.Command{
		Use:   "version",
		Short: "Print the version",
		RunE: func(cmd *cobra.Command, _ []string) error {
			_, err := fmt.Fprintf(cmd.OutOrStdout(), "Sekeve %s\n", version.Version)
			return err
		},
	})

	root.AddCommand(client.NewAddCmd())
	root.AddCommand(client.NewGetCmd())
	root.AddCommand(client.NewListCmd())
	root.AddCommand(client.NewEditCmd())
	root.AddCommand(client.NewRmCmd())
	root.AddCommand(client.NewSearchCmd())
	root.AddCommand(client.NewDmenuCmd())
	root.AddCommand(client.NewInitCmd())
	root.AddCommand(client.NewImportCmd())
	root.AddCommand(client.NewInjectCmd())
	root.AddCommand(client.NewPINCmd())
	registerPINPrompt(root)

	serverCmd := &cobra.Command{Use: "server", Short: "Server management commands"}
	serverCmd.AddCommand(server.NewStartCmd())
	serverCmd.AddCommand(server.NewInitCmd())
	root.AddCommand(serverCmd)

	return root
}

func Execute() {
	ctx := context.Background()
	root := NewRootCmd()
	root.SetContext(ctx)
	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}
