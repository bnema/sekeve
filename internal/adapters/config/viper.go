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

type ViperConfig struct {
	v *viper.Viper
}

func NewViperConfig(ctx context.Context) (*ViperConfig, error) {
	log := zerowrap.FromCtx(ctx)

	v := viper.New()
	v.SetConfigName("config")
	v.SetConfigType("toml")
	v.AddConfigPath(configDir())

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
		// No config file is fine, use defaults
	}

	return &ViperConfig{v: v}, nil
}

func (c *ViperConfig) ServerAddr(_ context.Context) string {
	return c.v.GetString("server_addr")
}

func (c *ViperConfig) GPGKeyID(_ context.Context) string {
	return c.v.GetString("gpg_key_id")
}

func (c *ViperConfig) SessionToken(_ context.Context) (string, error) {
	// Session is stored separately in a yaml file (not in the toml config)
	// because it changes frequently and we don't want to rewrite the config file
	path := filepath.Join(configDir(), "session")
	data, err := os.ReadFile(path)
	if err != nil {
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
	dir := configDir()
	if err := os.MkdirAll(dir, 0700); err != nil {
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
	return os.WriteFile(filepath.Join(dir, "session"), data, 0600)
}

// SetOverride allows CLI flags to override config file values.
func (c *ViperConfig) SetOverride(key, value string) {
	c.v.Set(key, value)
}

func configDir() string {
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
