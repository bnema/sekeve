// Package cliconfig holds shared CLI state (flags, session helpers)
// that must be accessible from sub-command packages without creating import cycles.
package cliconfig

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"runtime/secret"
	"sync"
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

const pinPromptKey ctxKey = "pinPrompt"

// PINPromptFromCtx retrieves the PINPromptPort stored in the context.
// Panics if WithPINPrompt was not called before ConnectAndAuth.
func PINPromptFromCtx(ctx context.Context) port.PINPromptPort {
	p, ok := ctx.Value(pinPromptKey).(port.PINPromptPort)
	if !ok {
		panic("PINPromptFromCtx: no PINPromptPort in context; ensure WithPINPrompt was called before ConnectAndAuth")
	}
	return p
}

// WithPINPrompt returns a new context with the given PINPromptPort embedded.
func WithPINPrompt(ctx context.Context, p port.PINPromptPort) context.Context {
	return context.WithValue(ctx, pinPromptKey, p)
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
		prompt := PINPromptFromCtx(ctx)
		var unlockErr error

		var mu sync.Mutex // protects attempts and authResult across goroutines
		attempts := 0
		validate := func(vctx context.Context, pin string) error {
			// Wrap in secret.Do because this callback may be invoked from a
			// goroutine spawned by the GUI adapter (GTK event loop), which
			// is not covered by the caller's secret.Do scope.
			var result error
			secret.Do(func() {
				mu.Lock()
				defer mu.Unlock()

				attempts++
				token, expiresAt, vErr := clientApp.Sync.Unlock(vctx, authResult.UnlockTicket, pin)
				if vErr == nil {
					cacheSession(token, expiresAt)
					return
				}

				st, ok := status.FromError(vErr)
				if !ok {
					result = &port.PINFatalError{Err: vErr}
					return
				}

				switch st.Code() {
				case codes.PermissionDenied:
					if attempts >= 3 {
						result = &port.PINFatalError{Err: fmt.Errorf("too many failed PIN attempts")}
						return
					}
					// User-facing message, intentionally capitalized.
					result = fmt.Errorf("%s", port.DefaultPINError)
				case codes.Unauthenticated:
					authRes, authErr := clientApp.Vault.Authenticate(vctx)
					if authErr != nil {
						result = &port.PINFatalError{Err: fmt.Errorf("re-authentication failed: %w", authErr)}
						return
					}
					authResult = authRes
					attempts = 0
					// User-facing message, intentionally capitalized.
					result = fmt.Errorf("Session expired, enter PIN again")
				case codes.ResourceExhausted:
					result = fmt.Errorf("%s", st.Message())
				default:
					result = &port.PINFatalError{Err: vErr}
				}
			})
			return result
		}

		unlockErr = prompt.PromptForPIN(ctx, validate)

		if unlockErr != nil {
			_ = clientApp.Close(ctx)
			if errors.Is(unlockErr, port.ErrPINPromptCancelled) {
				return nil, unlockErr
			}
			if !prompt.IsTTY() {
				SendNotification(ctx, "PIN unlock failed")
			}
			return nil, log.WrapErr(unlockErr, "unlock failed")
		}

		return clientApp, nil
	}

	cacheSession(authResult.Token, authResult.ExpiresAt)
	return clientApp, nil
}
