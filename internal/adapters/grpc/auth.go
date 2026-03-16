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

// AuthManager handles challenge-response authentication and session token management.
type AuthManager struct {
	mu        sync.Mutex
	nonces    map[string]nonceEntry
	sessions  map[string]sessionEntry
	gpgPubKey []byte
}

// NewAuthManager creates a new AuthManager with the provided GPG public key.
func NewAuthManager(gpgPubKey []byte) *AuthManager {
	return &AuthManager{
		nonces:    make(map[string]nonceEntry),
		sessions:  make(map[string]sessionEntry),
		gpgPubKey: gpgPubKey,
	}
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

// VerifyNonce verifies that a nonce exists and has not expired. On success it
// generates a session token (32-byte hex, 1h TTL) and returns the token and its
// expiry time.
func (a *AuthManager) VerifyNonce(ctx context.Context, nonce string) (string, time.Time, error) {
	log := zerowrap.FromCtx(ctx)

	a.mu.Lock()
	defer a.mu.Unlock()

	entry, ok := a.nonces[nonce]
	if !ok {
		return "", time.Time{}, status.Error(codes.Unauthenticated, "nonce not found")
	}
	if time.Now().After(entry.expiresAt) {
		delete(a.nonces, nonce)
		return "", time.Time{}, status.Error(codes.Unauthenticated, "nonce expired")
	}
	delete(a.nonces, nonce)

	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", time.Time{}, log.WrapErr(err, "failed to generate session token")
	}
	token := hex.EncodeToString(buf)
	expiry := time.Now().Add(sessionTTL)
	a.sessions[token] = sessionEntry{expiresAt: expiry}

	return token, expiry, nil
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
