package config_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/bnema/sekeve/internal/adapters/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestViperConfig_Defaults(t *testing.T) {
	// Point config dir to empty temp dir so no config file is found
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	ctx := context.Background()

	cfg, err := config.NewViperConfig(ctx)
	require.NoError(t, err)

	assert.Equal(t, "localhost:50051", cfg.ServerAddr(ctx))
	assert.Equal(t, "", cfg.GPGKeyID(ctx))
}

func TestViperConfig_FromTOMLFile(t *testing.T) {
	tmpDir := t.TempDir()
	configDir := filepath.Join(tmpDir, "sekeve")
	require.NoError(t, os.MkdirAll(configDir, 0700))

	tomlContent := `server_addr = "remote.example.com:9090"
gpg_key_id = "alice@example.com"
`
	require.NoError(t, os.WriteFile(filepath.Join(configDir, "config.toml"), []byte(tomlContent), 0600))

	t.Setenv("XDG_CONFIG_HOME", tmpDir)
	ctx := context.Background()

	cfg, err := config.NewViperConfig(ctx)
	require.NoError(t, err)

	assert.Equal(t, "remote.example.com:9090", cfg.ServerAddr(ctx))
	assert.Equal(t, "alice@example.com", cfg.GPGKeyID(ctx))
}

func TestViperConfig_EnvVarOverride(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("SEKEVE_SERVER_ADDR", "env.example.com:1234")
	t.Setenv("SEKEVE_GPG_KEY_ID", "bob@example.com")
	ctx := context.Background()

	cfg, err := config.NewViperConfig(ctx)
	require.NoError(t, err)

	assert.Equal(t, "env.example.com:1234", cfg.ServerAddr(ctx))
	assert.Equal(t, "bob@example.com", cfg.GPGKeyID(ctx))
}

func TestViperConfig_SetOverride(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	ctx := context.Background()

	cfg, err := config.NewViperConfig(ctx)
	require.NoError(t, err)

	cfg.SetOverride("server_addr", "override.example.com:5555")
	assert.Equal(t, "override.example.com:5555", cfg.ServerAddr(ctx))
}

func TestViperConfig_SessionTokenRoundTrip(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)

	// Ensure sekeve config dir exists
	require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, "sekeve"), 0700))

	ctx := context.Background()
	cfg, err := config.NewViperConfig(ctx)
	require.NoError(t, err)

	// No session yet
	_, err = cfg.SessionToken(ctx)
	assert.Error(t, err)

	// Save a session with 1 hour TTL
	require.NoError(t, cfg.SaveSessionToken(ctx, "my-token-123", 3600))

	// Read it back
	token, err := cfg.SessionToken(ctx)
	require.NoError(t, err)
	assert.Equal(t, "my-token-123", token)
}

func TestViperConfig_SessionTokenExpired(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)
	require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, "sekeve"), 0700))

	ctx := context.Background()
	cfg, err := config.NewViperConfig(ctx)
	require.NoError(t, err)

	// Save with 0 TTL (already expired)
	require.NoError(t, cfg.SaveSessionToken(ctx, "expired-token", 0))

	_, err = cfg.SessionToken(ctx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "expired")
}
