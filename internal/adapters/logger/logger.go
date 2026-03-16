package logger

import (
	"context"

	"github.com/bnema/zerowrap"
)

// New creates a logger configured from environment variables with fallback defaults.
// Environment variables:
//   - SEKEVE_LOG_LEVEL: trace, debug, info, warn, error (default: info)
//   - SEKEVE_LOG_FORMAT: console, json (default: console)
func New(ctx context.Context) (zerowrap.Logger, context.Context) {
	log := zerowrap.NewFromEnv("SEKEVE")
	ctx = zerowrap.WithCtx(ctx, log)
	return log, ctx
}
