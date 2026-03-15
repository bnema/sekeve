package config

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/bnema/sekeve/internal/port"
	"github.com/bnema/zerowrap"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"
)

// ErrNoSession indicates no session token is stored.
var ErrNoSession = errors.New("no session token stored")

type ViperConfig struct {
	v      *viper.Viper
	cfgDir string
}

func NewViperConfig(ctx context.Context, xdg port.XDGPort) (*ViperConfig, error) {
	log := zerowrap.FromCtx(ctx)

	dir, err := xdg.ConfigDir()
	if err != nil {
		return nil, log.WrapErr(err, "failed to resolve config directory")
	}

	v := viper.New()
	v.SetConfigName("config")
	v.SetConfigType("toml")
	v.AddConfigPath(dir)

	// Defaults
	v.SetDefault("server_addr", "localhost:50051")
	v.SetDefault("gpg_key_id", "")

	// Env var overrides
	v.SetEnvPrefix("SEKEVE")
	v.AutomaticEnv()

	if err := v.ReadInConfig(); err != nil {
		var notFound viper.ConfigFileNotFoundError
		if !errors.As(err, &notFound) {
			return nil, log.WrapErr(err, "failed to read config")
		}
	}

	return &ViperConfig{v: v, cfgDir: dir}, nil
}

func (c *ViperConfig) ServerAddr(_ context.Context) string {
	return c.v.GetString("server_addr")
}

func (c *ViperConfig) GPGKeyID(_ context.Context) string {
	return c.v.GetString("gpg_key_id")
}

func (c *ViperConfig) SessionToken(_ context.Context) (string, error) {
	path := filepath.Join(c.cfgDir, "session")
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", ErrNoSession
		}
		return "", err
	}
	var session struct {
		Token     string    `yaml:"token"`
		ExpiresAt time.Time `yaml:"expires_at"`
	}
	if err := yaml.Unmarshal(data, &session); err != nil {
		return "", err
	}
	if time.Now().After(session.ExpiresAt) {
		return "", fmt.Errorf("session expired")
	}
	return session.Token, nil
}

func (c *ViperConfig) SaveSessionToken(_ context.Context, token string, ttl int64) error {
	if err := os.MkdirAll(c.cfgDir, 0700); err != nil {
		return err
	}
	session := struct {
		Token     string    `yaml:"token"`
		ExpiresAt time.Time `yaml:"expires_at"`
	}{
		Token:     token,
		ExpiresAt: time.Now().Add(time.Duration(ttl) * time.Second),
	}
	data, err := yaml.Marshal(session)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(c.cfgDir, "session"), data, 0600)
}

// SetOverride allows CLI flags to override config file values.
func (c *ViperConfig) SetOverride(key, value string) {
	c.v.Set(key, value)
}

// IsUnconfigured returns true when no config file exists and gpg_key_id
// is still the empty default - meaning the user has never set up the client.
func (c *ViperConfig) IsUnconfigured() bool {
	return c.v.GetString("gpg_key_id") == "" && c.v.ConfigFileUsed() == ""
}

// WriteConfig creates the config directory and writes a config.toml with
// the given server address and GPG key ID.
func (c *ViperConfig) WriteConfig(serverAddr, gpgKeyID string) error {
	if err := os.MkdirAll(c.cfgDir, 0700); err != nil {
		return fmt.Errorf("failed to create config dir: %w", err)
	}

	// Update in-memory config so the current session uses the new values.
	c.v.Set("server_addr", serverAddr)
	c.v.Set("gpg_key_id", gpgKeyID)

	path := filepath.Join(c.cfgDir, "config.toml")
	if err := c.v.WriteConfigAs(path); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}

	// Ensure restrictive permissions.
	if err := os.Chmod(path, 0600); err != nil {
		return fmt.Errorf("failed to set config permissions: %w", err)
	}

	return nil
}

// ConfigPath returns the path to the config file.
func (c *ViperConfig) ConfigPath() string {
	return filepath.Join(c.cfgDir, "config.toml")
}
