package logger_test

import (
	"context"
	"testing"

	"github.com/bnema/sekeve/internal/adapters/logger"
	"github.com/bnema/zerowrap"
	"github.com/stretchr/testify/assert"
)

func TestNew_DefaultLevel(t *testing.T) {
	// No env vars set, should default to info level
	ctx := context.Background()
	log, newCtx := logger.New(ctx)

	assert.NotNil(t, log)
	assert.NotEqual(t, ctx, newCtx, "context should be enriched with logger")

	// Logger should be retrievable from context
	retrieved := zerowrap.FromCtx(newCtx)
	assert.NotNil(t, retrieved)
}

func TestNew_EnvOverride(t *testing.T) {
	t.Setenv("SEKEVE_LOG_LEVEL", "debug")
	t.Setenv("SEKEVE_LOG_FORMAT", "json")

	ctx := context.Background()
	log, _ := logger.New(ctx)

	assert.NotNil(t, log)
}

func TestNew_InvalidLevel_FallsBack(t *testing.T) {
	t.Setenv("SEKEVE_LOG_LEVEL", "garbage")

	ctx := context.Background()
	log, _ := logger.New(ctx)

	// Should not panic, falls back to a default
	assert.NotNil(t, log)
}
