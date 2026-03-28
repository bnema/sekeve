package client

import (
	"fmt"
	"os"

	"github.com/bnema/sekeve/internal/adapters/cli/cliconfig"
	"github.com/bnema/sekeve/internal/adapters/cli/styles"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

func NewPINCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "pin",
		Short: "Manage unlock PIN",
	}
	cmd.AddCommand(newPINSetCmd())
	cmd.AddCommand(newPINChangeCmd())
	cmd.AddCommand(newPINDisableCmd())
	return cmd
}

func readPIN(prompt string) (string, error) {
	fmt.Fprint(os.Stderr, prompt)
	pin, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Fprintln(os.Stderr)
	if err != nil {
		return "", err
	}
	return string(pin), nil
}

func newPINSetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "set",
		Short: "Set a new unlock PIN (4-6 digits)",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			cfg := cliconfig.ConfigFromCmd(cmd)
			clientApp, err := cliconfig.ConnectAndAuth(ctx, cfg)
			if err != nil {
				return err
			}
			defer clientApp.Close(ctx)

			hasPIN, err := clientApp.Sync.HasPIN(ctx)
			if err != nil {
				return err
			}
			if hasPIN {
				_ = styles.RenderError(os.Stderr, fmt.Errorf("PIN already set; use 'sekeve pin change' instead"))
				return fmt.Errorf("PIN already set")
			}

			pin, err := readPIN("New PIN: ")
			if err != nil {
				return err
			}
			confirm, err := readPIN("Confirm PIN: ")
			if err != nil {
				return err
			}
			if pin != confirm {
				return fmt.Errorf("PINs do not match")
			}
			if len(pin) < 4 || len(pin) > 6 {
				return fmt.Errorf("PIN must be 4-6 digits")
			}

			if err := clientApp.Sync.SetPIN(ctx, "", pin); err != nil {
				_ = styles.RenderError(os.Stderr, err)
				return err
			}
			return styles.RenderSuccess(os.Stderr, "PIN set successfully")
		},
	}
}

func newPINChangeCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "change",
		Short: "Change the unlock PIN",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			cfg := cliconfig.ConfigFromCmd(cmd)
			clientApp, err := cliconfig.ConnectAndAuth(ctx, cfg)
			if err != nil {
				return err
			}
			defer clientApp.Close(ctx)

			currentPIN, err := readPIN("Current PIN: ")
			if err != nil {
				return err
			}
			newPIN, err := readPIN("New PIN: ")
			if err != nil {
				return err
			}
			confirm, err := readPIN("Confirm new PIN: ")
			if err != nil {
				return err
			}
			if newPIN != confirm {
				return fmt.Errorf("PINs do not match")
			}
			if len(newPIN) < 4 || len(newPIN) > 6 {
				return fmt.Errorf("PIN must be 4-6 digits")
			}

			if err := clientApp.Sync.SetPIN(ctx, currentPIN, newPIN); err != nil {
				_ = styles.RenderError(os.Stderr, err)
				return err
			}
			return styles.RenderSuccess(os.Stderr, "PIN changed successfully")
		},
	}
}

func newPINDisableCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "disable",
		Short: "Remove the unlock PIN",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			cfg := cliconfig.ConfigFromCmd(cmd)
			clientApp, err := cliconfig.ConnectAndAuth(ctx, cfg)
			if err != nil {
				return err
			}
			defer clientApp.Close(ctx)

			_ = styles.RenderError(os.Stderr, fmt.Errorf("not implemented yet"))
			return fmt.Errorf("not implemented")
		},
	}
}
