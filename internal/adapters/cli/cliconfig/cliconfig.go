// Package cliconfig holds shared CLI state (flags, session helpers)
// that must be accessible from sub-command packages without creating import cycles.
package cliconfig

import (
	"context"
	"errors"
	"fmt"
	"os"
	"runtime/secret"
	"sync"
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

const guiKey ctxKey = "gui"

// GUIFromCtx retrieves the GUIPort stored in the context.
func GUIFromCtx(ctx context.Context) port.GUIPort {
	g, ok := ctx.Value(guiKey).(port.GUIPort)
	if !ok {
		panic("GUIFromCtx: no GUIPort in context; ensure WithGUI was called")
	}
	return g
}

// WithGUI returns a new context with the given GUIPort embedded.
func WithGUI(ctx context.Context, g port.GUIPort) context.Context {
	return context.WithValue(ctx, guiKey, g)
}

const cryptoKey ctxKey = "crypto"

// CryptoFromCtx retrieves the CryptoPort stored in the context.
// Panics if WithCrypto was not called before ConnectAndAuth.
func CryptoFromCtx(ctx context.Context) port.CryptoPort {
	c, ok := ctx.Value(cryptoKey).(port.CryptoPort)
	if !ok {
		panic("CryptoFromCtx: no CryptoPort in context; ensure WithCrypto was called before ConnectAndAuth")
	}
	return c
}

// WithCrypto returns a new context with the given CryptoPort embedded.
func WithCrypto(ctx context.Context, c port.CryptoPort) context.Context {
	return context.WithValue(ctx, cryptoKey, c)
}

const notifyKey ctxKey = "notify"

// NotifyFromCtx retrieves the NotificationPort stored in the context.
// Returns a silent no-op if none was set (notifications are non-critical).
func NotifyFromCtx(ctx context.Context) port.NotificationPort {
	if n, ok := ctx.Value(notifyKey).(port.NotificationPort); ok {
		return n
	}
	return noopNotifier{}
}

// noopNotifier is an inline no-op so cliconfig doesn't import the adapter package.
type noopNotifier struct{}

func (noopNotifier) Notify(context.Context, string, string, port.Urgency, string) error { return nil }
func (noopNotifier) Close() error                                                       { return nil }

// WithNotify returns a new context with the given NotificationPort embedded.
func WithNotify(ctx context.Context, n port.NotificationPort) context.Context {
	return context.WithValue(ctx, notifyKey, n)
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

// ConnectAndAuthDeferPIN is like ConnectAndAuth but, when PIN is required,
// it stores the validate function on the GUI adapter via SetPendingPIN instead
// of immediately prompting. The PIN will be validated inside ShowOmnibox.
// Use this for the omnibox command to keep both prompts in one GTK application.
func ConnectAndAuthDeferPIN(ctx context.Context, cfg port.ConfigPort, gui port.GUIPort) (*app.ClientApp, error) {
	log := zerowrap.FromCtx(ctx)

	if cfg.IsUnconfigured() {
		return nil, fmt.Errorf("client not configured; run 'sekeve init' first")
	}

	crypto := CryptoFromCtx(ctx)
	clientApp, err := app.NewClientApp(ctx, cfg, crypto)
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

	cacheSession := func(tok string, expiresAt time.Time) {
		clientApp.Sync.SetToken(tok)
		if saveErr := cfg.SaveSessionToken(ctx, tok, int64(time.Until(expiresAt).Seconds())); saveErr != nil {
			log.Warn().Err(saveErr).Msg("failed to cache session")
		}
	}

	if authResult.RequiresPIN {
		validate := makeDeferredUnlockValidator(clientApp, authResult, cacheSession)
		gui.SetPendingPIN(validate)
		return clientApp, nil
	}

	cacheSession(authResult.Token, authResult.ExpiresAt)
	return clientApp, nil
}

// makeDeferredUnlockValidator creates a PIN validation callback for the
// deferred PIN flow (omnibox). Uses domain errors matching the pattern
// in app.ClientApp.unlockWithPIN.
func makeDeferredUnlockValidator(
	clientApp *app.ClientApp,
	authResult *port.AuthResult,
	cacheSession func(token string, expiresAt time.Time),
) port.PINValidateFunc {
	var mu sync.Mutex
	attempts := 0

	return func(vctx context.Context, pin string) error {
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

			switch {
			case errors.Is(vErr, port.ErrPermissionDenied):
				if attempts >= 3 {
					result = &port.PINFatalError{Err: fmt.Errorf("too many failed PIN attempts")}
					return
				}
				// User-facing message, intentionally capitalized.
				result = fmt.Errorf("%s", port.DefaultPINError)
			case errors.Is(vErr, port.ErrSessionExpired):
				authRes, authErr := clientApp.Vault.Authenticate(vctx)
				if authErr != nil {
					result = &port.PINFatalError{Err: fmt.Errorf("re-authentication failed: %w", authErr)}
					return
				}
				*authResult = *authRes
				attempts = 0
				// User-facing message, intentionally capitalized.
				result = fmt.Errorf("Session expired, enter PIN again") //nolint:staticcheck
			case errors.Is(vErr, port.ErrRateLimited):
				result = fmt.Errorf("%v", vErr)
			default:
				result = &port.PINFatalError{Err: vErr}
			}
		})
		return result
	}
}

func ConnectAndAuth(ctx context.Context, cfg port.ConfigPort) (*app.ClientApp, error) {
	log := zerowrap.FromCtx(ctx)

	// Check if client needs onboarding.
	if cfg.IsUnconfigured() {
		return nil, fmt.Errorf("client not configured; run 'sekeve init' first")
	}

	crypto := CryptoFromCtx(ctx)
	clientApp, err := app.NewClientApp(ctx, cfg, crypto)
	if err != nil {
		return nil, log.WrapErr(err, "failed to connect")
	}

	prompt := PINPromptFromCtx(ctx)
	notify := NotifyFromCtx(ctx)
	if err := clientApp.AuthenticateSession(ctx, prompt, notify); err != nil {
		_ = clientApp.Close(ctx)
		return nil, err
	}


	return clientApp, nil
}
