// Package cliconfig holds shared CLI state (flags, session helpers)
// that must be accessible from sub-command packages without creating import cycles.
package cliconfig

import (
	"context"
	"fmt"
	"os"

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
