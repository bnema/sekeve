package config

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

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

func NewViperConfig(ctx context.Context) (*ViperConfig, error) {
	log := zerowrap.FromCtx(ctx)

	dir := resolveConfigDir()

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

func resolveConfigDir() string {
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
