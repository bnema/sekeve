// Package cliconfig holds shared CLI state (flags, session helpers)
// that must be accessible from sub-command packages without creating import cycles.
package cliconfig

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/bnema/sekeve/internal/app"
	"github.com/bnema/sekeve/internal/port"
	"github.com/bnema/zerowrap"
	"github.com/spf13/cobra"
	"golang.org/x/term"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
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

// ReadPassword prints the prompt to stderr, reads a password without echo, and returns it.
func ReadPassword(prompt string) (string, error) {
	fmt.Fprint(os.Stderr, prompt)
	b, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Fprintln(os.Stderr)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// SendNotification sends a best-effort desktop notification via notify-send (Linux only).
// Silently ignored on systems where notify-send is unavailable.
func SendNotification(ctx context.Context, body string) {
	_ = exec.CommandContext(ctx, "notify-send", "-a", "sekeve", "-i", "dialog-password", "Sekeve", body).Run()
}

// execPINPrompt spawns "sekeve pin-prompt" as a subprocess and reads the PIN from stdout.
func execPINPrompt(ctx context.Context, errorMode bool, message string) (string, error) {
	exePath, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("failed to resolve executable path: %w", err)
	}

	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	args := []string{"pin-prompt"}
	if errorMode {
		args = append(args, "--error")
	}
	if message != "" {
		args = append(args, "--message", message)
	}

	cmd := exec.CommandContext(ctx, exePath, args...)
	cmd.Stderr = os.Stderr
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("PIN prompt cancelled or failed: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
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

	cacheSession := func(token string, expiresAt time.Time) {
		clientApp.Sync.SetToken(token)
		if saveErr := cfg.SaveSessionToken(ctx, token, int64(time.Until(expiresAt).Seconds())); saveErr != nil {
			log.Warn().Err(saveErr).Msg("failed to cache session")
		}
	}

	if authResult.RequiresPIN {
		isTTY := term.IsTerminal(int(os.Stdin.Fd()))

		readPIN := func(errorMode bool, message string) (string, error) {
			if isTTY {
				if message != "" {
					fmt.Fprintln(os.Stderr, message)
				} else if errorMode {
					fmt.Fprintln(os.Stderr, "Incorrect PIN, please try again.")
				}
				return ReadPassword("Unlock PIN: ")
			}
			return execPINPrompt(ctx, errorMode, message)
		}

		pin, pinErr := readPIN(false, "")
		if pinErr != nil {
			clientApp.Close(ctx)
			return nil, fmt.Errorf("failed to read PIN: %w", pinErr)
		}

		var token string
		var expiresAt time.Time
	retryLoop:
		for attempts := 0; attempts < 3; attempts++ {
			token, expiresAt, err = clientApp.Sync.Unlock(ctx, authResult.UnlockTicket, pin)
			if err == nil {
				break
			}

			st, ok := status.FromError(err)
			if !ok {
				break
			}

			switch st.Code() {
			case codes.PermissionDenied:
				pin, pinErr = readPIN(true, "")
			case codes.Unauthenticated:
				var authErr error
				authResult, authErr = clientApp.Vault.Authenticate(ctx)
				if authErr != nil {
					err = fmt.Errorf("re-authentication failed: %w", authErr)
					break retryLoop
				}
				pin, pinErr = readPIN(true, "Session expired, enter PIN again")
			case codes.ResourceExhausted:
				pin, pinErr = readPIN(true, st.Message())
			default:
				pinErr = err
			}

			if pinErr != nil {
				err = pinErr
				break
			}
		}

		if err != nil {
			clientApp.Close(ctx)
			if !isTTY {
				SendNotification(ctx, "PIN unlock failed")
			}
			return nil, log.WrapErr(err, "unlock failed")
		}

		cacheSession(token, expiresAt)
		return clientApp, nil
	}

	cacheSession(authResult.Token, authResult.ExpiresAt)
	return clientApp, nil
}
