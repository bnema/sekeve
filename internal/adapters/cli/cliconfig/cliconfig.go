// Package cliconfig holds shared CLI state (flags, session helpers)
// that must be accessible from sub-command packages without creating import cycles.
package cliconfig

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/bnema/sekeve/internal/app"
	"github.com/bnema/sekeve/internal/port"
	"github.com/bnema/zerowrap"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

// CLI flag vars — overrides for config file values.
var (
	ServerAddr string
	GPGKeyID   string
	JSONOutput bool
)

type ctxKey string

const configKey ctxKey = "config"

// ConfigFromCmd retrieves the ConfigPort stored in the command's context.
// Panics if WithConfig was not called in PersistentPreRunE.
func ConfigFromCmd(cmd *cobra.Command) port.ConfigPort {
	cfg, ok := cmd.Context().Value(configKey).(port.ConfigPort)
	if !ok {
		panic("ConfigFromCmd: no config in context; ensure WithConfig was called in PersistentPreRunE")
	}
	return cfg
}

// WithConfig returns a new context with the given ConfigPort embedded.
func WithConfig(ctx context.Context, cfg port.ConfigPort) context.Context {
	return context.WithValue(ctx, configKey, cfg)
}

func ConnectAndAuth(ctx context.Context, cfg port.ConfigPort) (*app.ClientApp, error) {
	log := zerowrap.FromCtx(ctx)

	// Check if client needs onboarding.
	if cfg.IsUnconfigured() {
		return nil, fmt.Errorf("client not configured; run 'sekeve init' first")
	}

	clientApp, err := app.NewClientApp(ctx, cfg)
	if err != nil {
		return nil, log.WrapErr(err, "failed to connect")
	}

	// Try cached session first.
	token, err := cfg.SessionToken(ctx)
	if err == nil {
		clientApp.Sync.SetToken(token)
		return clientApp, nil
	}

	// No valid cached session — authenticate via GPG challenge-response.
	authResult, err := clientApp.Vault.Authenticate(ctx)
	if err != nil {
		if closeErr := clientApp.Close(ctx); closeErr != nil {
			log.Warn().Err(closeErr).Msg("failed to close client app after auth failure")
		}
		return nil, log.WrapErr(err, "authentication failed")
	}

	if authResult.RequiresPIN {
		// Prompt for PIN.
		fmt.Fprint(os.Stderr, "Unlock PIN: ")
		pinBytes, pinErr := term.ReadPassword(int(os.Stdin.Fd()))
		fmt.Fprintln(os.Stderr)
		if pinErr != nil {
			clientApp.Close(ctx)
			return nil, fmt.Errorf("failed to read PIN: %w", pinErr)
		}

		token, expiresAt, unlockErr := clientApp.Sync.Unlock(ctx, authResult.UnlockTicket, string(pinBytes))
		clear(pinBytes)
		if unlockErr != nil {
			clientApp.Close(ctx)
			return nil, log.WrapErr(unlockErr, "unlock failed")
		}

		clientApp.Sync.SetToken(token)
		if saveErr := cfg.SaveSessionToken(ctx, token, int64(time.Until(expiresAt).Seconds())); saveErr != nil {
			log.Warn().Err(saveErr).Msg("failed to cache session")
		}
		return clientApp, nil
	}

	// No PIN required — use token directly.
	clientApp.Sync.SetToken(authResult.Token)
	if saveErr := cfg.SaveSessionToken(ctx, authResult.Token, int64(time.Until(authResult.ExpiresAt).Seconds())); saveErr != nil {
		log.Warn().Err(saveErr).Msg("failed to cache session")
	}

	return clientApp, nil
}
