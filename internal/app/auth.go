package app

import (
	"context"
	"errors"
	"fmt"
	"runtime/secret"
	"sync"
	"time"

	"github.com/bnema/sekeve/internal/port"
	"github.com/bnema/zerowrap"
)

// AuthenticateSession performs the full authentication flow: tries a cached
// session, falls back to GPG challenge-response, and handles PIN unlock if
// required. On success the gRPC client token is set and the session is cached.
func (a *ClientApp) AuthenticateSession(ctx context.Context, prompt port.PINPromptPort, notify port.NotificationPort) error {
	log := zerowrap.FromCtx(ctx)

	// Try cached session first.
	token, err := a.Config.SessionToken(ctx)
	if err == nil {
		a.Sync.SetToken(token)
		return nil
	}

	// No valid cached session — authenticate via GPG challenge-response.
	authResult, err := a.Vault.Authenticate(ctx)
	if err != nil {
		return log.WrapErr(err, "authentication failed")
	}

	cacheSession := func(tok string, expiresAt time.Time) {
		a.Sync.SetToken(tok)
		if saveErr := a.Config.SaveSessionToken(ctx, tok, int64(time.Until(expiresAt).Seconds())); saveErr != nil {
			log.Warn().Err(saveErr).Msg("failed to cache session")
		}
	}

	if !authResult.RequiresPIN {
		cacheSession(authResult.Token, authResult.ExpiresAt)
		return nil
	}

	// PIN required — run the retry state machine.
	unlockErr := a.unlockWithPIN(ctx, prompt, authResult, cacheSession)
	if unlockErr != nil {
		if errors.Is(unlockErr, port.ErrPINPromptCancelled) {
			return unlockErr
		}
		if !prompt.IsTTY() {
			_ = notify.Notify(ctx, "Sekeve", "PIN unlock failed", port.UrgencyCritical, "dialog-error")
		}
		return log.WrapErr(unlockErr, "unlock failed")
	}
	return nil
}

// unlockWithPIN handles PIN prompt retries, re-authentication on session
// expiry, and rate-limit errors.
func (a *ClientApp) unlockWithPIN(
	ctx context.Context,
	prompt port.PINPromptPort,
	authResult *port.AuthResult,
	cacheSession func(string, time.Time),
) error {
	var mu sync.Mutex // protects attempts and authResult across goroutines
	attempts := 0

	validate := func(vctx context.Context, pin string) error {
		var result error
		// Wrap in secret.Do because this callback may be invoked from a
		// goroutine spawned by the GUI adapter (GTK event loop), which
		// is not covered by the caller's secret.Do scope.
		secret.Do(func() {
			mu.Lock()
			defer mu.Unlock()

			attempts++
			token, expiresAt, vErr := a.Sync.Unlock(vctx, authResult.UnlockTicket, pin)
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
				result = fmt.Errorf("%s", port.DefaultPINError)
			case errors.Is(vErr, port.ErrSessionExpired):
				authRes, authErr := a.Vault.Authenticate(vctx)
				if authErr != nil {
					result = &port.PINFatalError{Err: fmt.Errorf("re-authentication failed: %w", authErr)}
					return
				}
				authResult = authRes
				attempts = 0
				result = fmt.Errorf("Session expired, enter PIN again") //nolint:staticcheck // user-facing message
			case errors.Is(vErr, port.ErrRateLimited):
				result = fmt.Errorf("%v", vErr)
			default:
				result = &port.PINFatalError{Err: vErr}
			}
		})
		return result
	}

	return prompt.PromptForPIN(ctx, validate)
}
