package grpc

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync"
	"time"

	"github.com/bnema/zerowrap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

const (
	nonceTTL   = 30 * time.Second
	sessionTTL = time.Hour
)

type nonceEntry struct {
	expiresAt time.Time
}

type sessionEntry struct {
	expiresAt time.Time
}

// VerifyResult holds the outcome of a VerifyNonce call.
type VerifyResult struct {
	Token        string
	ExpiresAt    time.Time
	RequiresPIN  bool
	UnlockTicket string
}

// AuthManager handles challenge-response authentication and session token management.
type AuthManager struct {
	mu            sync.Mutex
	nonces        map[string]nonceEntry
	sessions      map[string]sessionEntry
	unlockTickets map[string]nonceEntry
	gpgPubKey     []byte
	pinConfigured bool
}

// NewAuthManager creates a new AuthManager with the provided GPG public key.
func NewAuthManager(gpgPubKey []byte) *AuthManager {
	return &AuthManager{
		nonces:        make(map[string]nonceEntry),
		sessions:      make(map[string]sessionEntry),
		unlockTickets: make(map[string]nonceEntry),
		gpgPubKey:     gpgPubKey,
	}
}

// SetPINConfigured records whether a PIN has been configured on the server.
func (am *AuthManager) SetPINConfigured(configured bool) {
	am.mu.Lock()
	defer am.mu.Unlock()
	am.pinConfigured = configured
}

// GPGPublicKey returns the stored GPG public key.
func (a *AuthManager) GPGPublicKey() []byte {
	return a.gpgPubKey
}

// GenerateChallenge generates a cryptographically random 32-byte nonce, stores it with a 30s TTL,
// and returns the hex-encoded nonce.
func (a *AuthManager) GenerateChallenge(ctx context.Context) (string, error) {
	log := zerowrap.FromCtx(ctx)

	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", log.WrapErr(err, "failed to generate nonce")
	}
	nonce := hex.EncodeToString(buf)

	a.mu.Lock()
	defer a.mu.Unlock()

	// Clean expired nonces to prevent unbounded memory growth.
	now := time.Now()
	for k, entry := range a.nonces {
		if now.After(entry.expiresAt) {
			delete(a.nonces, k)
		}
	}

	a.nonces[nonce] = nonceEntry{expiresAt: now.Add(nonceTTL)}

	return nonce, nil
}

// FormatChallenge returns the canonical challenge string for a given nonce.
func (a *AuthManager) FormatChallenge(nonce string) string {
	return fmt.Sprintf("sekeve-challenge:%s:%d", nonce, time.Now().Unix())
}

// generateToken returns a cryptographically random 32-byte hex string.
func generateToken() string {
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// VerifyNonce verifies that a nonce exists and has not expired. When PIN is
// configured it returns an unlock ticket instead of a session token. Without
// PIN it returns a session token directly.
func (a *AuthManager) VerifyNonce(ctx context.Context, nonce string) (*VerifyResult, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	entry, ok := a.nonces[nonce]
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "nonce not found")
	}
	if time.Now().After(entry.expiresAt) {
		delete(a.nonces, nonce)
		return nil, status.Error(codes.Unauthenticated, "nonce expired")
	}
	delete(a.nonces, nonce)

	if a.pinConfigured {
		ticket := generateToken()
		a.unlockTickets[ticket] = nonceEntry{
			expiresAt: time.Now().Add(nonceTTL),
		}
		return &VerifyResult{
			RequiresPIN:  true,
			UnlockTicket: ticket,
		}, nil
	}

	token := generateToken()
	expiresAt := time.Now().Add(sessionTTL)
	a.sessions[token] = sessionEntry{expiresAt: expiresAt}
	return &VerifyResult{
		Token:     token,
		ExpiresAt: expiresAt,
	}, nil
}

// RedeemUnlockTicket exchanges a one-time unlock ticket for a session token.
// The ticket is consumed on first use.
func (am *AuthManager) RedeemUnlockTicket(_ context.Context, ticket string) (string, time.Time, error) {
	am.mu.Lock()
	defer am.mu.Unlock()

	entry, ok := am.unlockTickets[ticket]
	if !ok {
		return "", time.Time{}, fmt.Errorf("invalid or expired unlock ticket")
	}
	delete(am.unlockTickets, ticket)

	if time.Now().After(entry.expiresAt) {
		return "", time.Time{}, fmt.Errorf("unlock ticket expired")
	}

	token := generateToken()
	expiresAt := time.Now().Add(sessionTTL)
	am.sessions[token] = sessionEntry{expiresAt: expiresAt}
	return token, expiresAt, nil
}

// SetTestToken sets a session token with a 24h expiry. Used only in tests.
func (a *AuthManager) SetTestToken(token string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.sessions[token] = sessionEntry{expiresAt: time.Now().Add(24 * time.Hour)}
}

// validateToken checks if a token is valid and not expired.
func (a *AuthManager) validateToken(token string) bool {
	a.mu.Lock()
	defer a.mu.Unlock()

	entry, ok := a.sessions[token]
	if !ok {
		return false
	}
	if time.Now().After(entry.expiresAt) {
		delete(a.sessions, token)
		return false
	}
	return true
}

// skipAuthMethods lists the full method names that do not require authentication.
var skipAuthMethods = map[string]bool{
	"/sekeve.v1.Sekeve/Authenticate":    true,
	"/sekeve.v1.Sekeve/VerifyChallenge": true,
	"/grpc.health.v1.Health/Check":      true,
}

// UnaryInterceptor returns a gRPC unary server interceptor that enforces token auth.
func (a *AuthManager) UnaryInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		if skipAuthMethods[info.FullMethod] {
			return handler(ctx, req)
		}

		md, ok := metadata.FromIncomingContext(ctx)
		if !ok {
			return nil, status.Error(codes.Unauthenticated, "missing metadata")
		}

		values := md.Get("authorization")
		if len(values) == 0 {
			return nil, status.Error(codes.Unauthenticated, "missing authorization token")
		}

		token := values[0]
		if !a.validateToken(token) {
			return nil, status.Error(codes.Unauthenticated, "invalid or expired token")
		}

		return handler(ctx, req)
	}
}
