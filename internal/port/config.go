package port

import "context"

// ConfigPort provides access to application configuration and session management.
type ConfigPort interface {
	// ServerAddr returns the gRPC server address.
	ServerAddr(ctx context.Context) string
	// GPGKeyID returns the configured GPG key identifier.
	GPGKeyID(ctx context.Context) string
	// SessionToken returns a cached session token if still valid.
	// Returns an error if no token exists or if it has expired.
	SessionToken(ctx context.Context) (string, error)
	// SaveSessionToken persists a session token. ttl is the duration in seconds
	// until expiry.
	SaveSessionToken(ctx context.Context, token string, ttl int64) error
}
