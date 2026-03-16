package port

import "context"

type ConfigPort interface {
	ServerAddr(ctx context.Context) string
	GPGKeyID(ctx context.Context) string
	SessionToken(ctx context.Context) (string, error)
	SaveSessionToken(ctx context.Context, token string, ttl int64) error
}
