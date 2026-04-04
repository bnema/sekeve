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

	clientApp, err := app.NewClientApp(ctx, cfg)
	if err != nil {
		return nil, log.WrapErr(err, "failed to connect")
	}

	token, err := cfg.SessionToken(ctx)
	if err == nil {
		clientApp.Sync.SetToken(token)
		return clientApp, nil
	}

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
		validate := makeUnlockValidator(ctx, clientApp, authResult, cacheSession)
		gui.SetPendingPIN(validate)
		return clientApp, nil
	}

	cacheSession(authResult.Token, authResult.ExpiresAt)
	return clientApp, nil
}

// makeUnlockValidator creates a PIN validation callback shared by
// ConnectAndAuth and ConnectAndAuthDeferPIN.
func makeUnlockValidator(
	ctx context.Context,
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
				*authResult = *authRes
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

		validate := makeUnlockValidator(ctx, clientApp, authResult, cacheSession)

		unlockErr = prompt.PromptForPIN(ctx, validate)

		if unlockErr != nil {
			_ = clientApp.Close(ctx)
			if errors.Is(unlockErr, port.ErrPINPromptCancelled) {
				return nil, unlockErr
			}
			if !prompt.IsTTY() {
				_ = NotifyFromCtx(ctx).Notify(ctx, "Sekeve", "PIN unlock failed", port.UrgencyCritical, "dialog-error")
			}
			return nil, log.WrapErr(unlockErr, "unlock failed")
		}

		return clientApp, nil
	}

	cacheSession(authResult.Token, authResult.ExpiresAt)
	return clientApp, nil
}
