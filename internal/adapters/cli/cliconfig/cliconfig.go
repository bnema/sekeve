// Package cliconfig holds shared CLI state (flags, config, session helpers)
// that must be accessible from sub-command packages without creating import cycles.
package cliconfig

import (
	"context"
	"os"
	"path/filepath"
	"time"

	"github.com/bnema/sekeve/internal/app"
	"github.com/bnema/zerowrap"
	"gopkg.in/yaml.v3"
)

// Flags are set by the root command persistent pre-run hook and read by sub-commands.
var (
	ServerAddr string
	GPGKeyID   string
	JSONOutput bool
)

// Config represents the on-disk config file.
type Config struct {
	ServerAddr string `yaml:"server_addr"`
	GPGKeyID   string `yaml:"gpg_key_id"`
}

// SessionCache is the cached auth session.
type SessionCache struct {
	Token     string    `yaml:"token"`
	ExpiresAt time.Time `yaml:"expires_at"`
}

func ConfigDir() string {
	dir, err := os.UserConfigDir()
	if err != nil {
		home, homeErr := os.UserHomeDir()
		if homeErr != nil {
			return ".sekeve"
		}
		return filepath.Join(home, ".config", "sekeve")
	}
	return filepath.Join(dir, "sekeve")
}

func LoadConfig() (*Config, error) {
	path := filepath.Join(ConfigDir(), "config.yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &Config{ServerAddr: "localhost:50051"}, nil
		}
		return nil, err
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func LoadSession() (*SessionCache, error) {
	path := filepath.Join(ConfigDir(), "session")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var s SessionCache
	if err := yaml.Unmarshal(data, &s); err != nil {
		return nil, err
	}
	return &s, nil
}

func SaveSession(s *SessionCache) error {
	dir := ConfigDir()
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	data, err := yaml.Marshal(s)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "session"), data, 0600)
}

func ConnectAndAuth(ctx context.Context, serverAddr, gpgKeyID string) (*app.ClientApp, error) {
	log := zerowrap.FromCtx(ctx)
	clientApp, err := app.NewClientApp(ctx, serverAddr, gpgKeyID)
	if err != nil {
		return nil, log.WrapErr(err, "failed to connect")
	}
	session, err := LoadSession()
	if err == nil && time.Now().Before(session.ExpiresAt) {
		clientApp.Sync.SetToken(session.Token)
		return clientApp, nil
	}
	token, err := clientApp.Vault.Authenticate(ctx)
	if err != nil {
		if closeErr := clientApp.Close(ctx); closeErr != nil {
			log.Warn().Err(closeErr).Msg("failed to close client app after auth failure")
		}
		return nil, log.WrapErr(err, "authentication failed")
	}
	if saveErr := SaveSession(&SessionCache{Token: token, ExpiresAt: time.Now().Add(1 * time.Hour)}); saveErr != nil {
		log.Warn().Err(saveErr).Msg("failed to cache session")
	}
	return clientApp, nil
}
