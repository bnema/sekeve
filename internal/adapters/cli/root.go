package cli

import (
	"context"
	"os"
	"os/signal"

	"github.com/bnema/sekeve/internal/adapters/cli/cliconfig"
	"github.com/bnema/sekeve/internal/adapters/cli/client"
	"github.com/bnema/sekeve/internal/adapters/cli/server"
	"github.com/bnema/zerowrap"
	"github.com/spf13/cobra"
)

func NewRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "sekeve",
		Short: "CLI secret manager with GPG encryption",
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := cliconfig.LoadConfig()
			if err != nil {
				return err
			}
			if cliconfig.ServerAddr == "" {
				cliconfig.ServerAddr = cfg.ServerAddr
			}
			if cliconfig.GPGKeyID == "" {
				cliconfig.GPGKeyID = cfg.GPGKeyID
			}
			if env := os.Getenv("SEKEVE_SERVER_ADDR"); env != "" {
				cliconfig.ServerAddr = env
			}
			if env := os.Getenv("SEKEVE_GPG_KEY_ID"); env != "" {
				cliconfig.GPGKeyID = env
			}
			logger := zerowrap.New(zerowrap.Config{Level: "info", Format: "console"})
			ctx, cancel := signal.NotifyContext(cmd.Context(), os.Interrupt)
			ctx = zerowrap.WithCtx(ctx, logger)
			cmd.SetContext(ctx)
			cobra.OnFinalize(func() { cancel() })
			return nil
		},
	}

	root.PersistentFlags().StringVar(&cliconfig.ServerAddr, "server", "", "server address")
	root.PersistentFlags().StringVar(&cliconfig.GPGKeyID, "gpg-key", "", "GPG key ID")
	root.PersistentFlags().BoolVar(&cliconfig.JSONOutput, "json", false, "output as JSON")

	root.AddCommand(client.NewAddCmd())
	root.AddCommand(client.NewGetCmd())
	root.AddCommand(client.NewListCmd())
	root.AddCommand(client.NewEditCmd())
	root.AddCommand(client.NewRmCmd())
	root.AddCommand(client.NewSearchCmd())
	root.AddCommand(client.NewDmenuCmd())

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
